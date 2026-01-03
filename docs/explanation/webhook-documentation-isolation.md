---
title: "Webhook and Documentation Isolation Strategy"
date: 2025-12-17
categories:
  - explanation
tags:
  - architecture
  - webhooks
  - security
---

# Webhook and Documentation Isolation Strategy

This document explains how DocBuilder prevents webhook endpoints from colliding with documentation content.

## Architecture: Multi-Server Design

DocBuilder uses a **defense-in-depth approach** with multiple isolated HTTP servers:

```
┌─────────────────────────────────────────────────────────┐
│                  DocBuilder Daemon                      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌────────────────┐  ┌────────────────┐                │
│  │  Docs Server   │  │ Webhook Server │                │
│  │  Port: 8080    │  │  Port: 8081    │                │
│  ├────────────────┤  ├────────────────┤                │
│  │ GET  /         │  │ POST /webhooks/│                │
│  │ GET  /docs/*   │  │      github    │                │
│  │ GET  /search/  │  │ POST /webhooks/│                │
│  │      index.json│  │      gitlab    │                │
│  └────────────────┘  │ POST /webhooks/│                │
│                      │      forgejo   │                │
│  ┌────────────────┐  └────────────────┘                │
│  │  Admin Server  │                                    │
│  │  Port: 8082    │  ┌────────────────┐                │
│  ├────────────────┤  │ LiveReload     │                │
│  │ GET  /health   │  │  Port: 8083    │                │
│  │ GET  /ready    │  │  (optional)    │                │
│  │ GET  /metrics  │  ├────────────────┤                │
│  │ POST /api/     │  │ GET  /sse      │                │
│  │      build/    │  └────────────────┘                │
│  │      trigger   │                                    │
│  └────────────────┘                                    │
└─────────────────────────────────────────────────────────┘
```

### Port Allocation

| Server | Default Port | Purpose | Collision Risk |
|--------|-------------|---------|----------------|
| **Docs** | 8080 | Serves Hugo-generated documentation | ERROR: None - separate server |
| **Webhook** | 8081 | Receives forge webhooks | ERROR: None - separate server |
| **Admin** | 8082 | Administrative API, health checks | ERROR: None - separate server |
| **LiveReload** | 8083 | Server-Sent Events for live reload | ERROR: None - separate server |

## Defense-in-Depth Layers

### Layer 1: Port Isolation (Primary Defense)

Each server runs on a **separate TCP port** with its own `http.Server` instance and request multiplexer (`http.ServeMux`). This provides complete isolation:

```go
// In internal/daemon/http_server.go
func (s *HTTPServer) Start(ctx context.Context) error {
    // Bind separate ports
    docsListener := net.Listen("tcp", ":8080")
    webhookListener := net.Listen("tcp", ":8081")
    adminListener := net.Listen("tcp", ":8082")
    
    // Start independent servers
    s.docsServer = &http.Server{Handler: docsHandler}
    s.webhookServer = &http.Server{Handler: webhookHandler}
    s.adminServer = &http.Server{Handler: adminHandler}
    
    go s.docsServer.Serve(docsListener)
    go s.webhookServer.Serve(webhookListener)
    go s.adminServer.Serve(adminListener)
}
```

**Collision Probability**: 0% - Requests to different ports go to completely different HTTP servers.

### Layer 2: Path Prefixing (Secondary Defense)

Even if servers were combined (they're not), webhook paths use reserved prefixes:

- `/webhooks/github`
- `/webhooks/gitlab`
- `/webhooks/forgejo`
- `/webhook` (generic)

These paths are unlikely to exist in Hugo documentation because:
- Hugo content typically lives in `/docs/`, `/blog/`, etc.
- The `/webhooks/` prefix is API-specific, not documentation content
- Hugo wouldn't generate these exact paths without explicit configuration

### Layer 3: HTTP Method Filtering (Tertiary Defense)

Webhook handlers **only accept POST requests**:

```go
func (h *WebhookHandlers) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        // Return 405 Method Not Allowed
        return
    }
    // Process webhook...
}
```

Documentation requests use **GET**, so even if a collision occurred:
- `GET /webhooks/github` → Documentation server (404 or docs file)
- `POST /webhooks/github` → Webhook server (webhook handler)

### Layer 4: Configuration Validation (Preventive)

Port binding validation happens at startup:

```go
// Pre-bind all ports before starting any servers
binds := []preBind{
    {name: "docs", port: config.Daemon.HTTP.DocsPort},
    {name: "webhook", port: config.Daemon.HTTP.WebhookPort},
    {name: "admin", port: config.Daemon.HTTP.AdminPort},
}

for _, bind := range binds {
    ln, err := net.Listen("tcp", fmt.Sprintf(":%d", bind.port))
    if err != nil {
        return fmt.Errorf("%s port %d: %w", bind.name, bind.port, err)
    }
    bind.ln = ln
}
```

If any port is already in use or if two services try to use the same port, **the daemon fails to start** with a clear error message.

## Additional Safeguards

### 1. Port Conflict Detection

DocBuilder validates that all configured ports are unique:

```yaml
daemon:
  http:
    docs_port: 8080
    webhook_port: 8081    # Must differ from docs_port
    admin_port: 8082      # Must differ from both above
    livereload_port: 8083 # Must differ from all above
```

If you accidentally configure the same port twice, the daemon will fail to start.

### 2. Firewall Recommendations

For production deployments, use firewall rules to restrict access:

```bash
# Allow public access to docs
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT

# Restrict webhook port to forge IPs only
iptables -A INPUT -p tcp --dport 8081 -s 140.82.112.0/20 -j ACCEPT  # GitHub
iptables -A INPUT -p tcp --dport 8081 -s 192.30.252.0/22 -j ACCEPT  # GitHub
# ... add GitLab/Forgejo IPs

# Restrict admin port to internal network
iptables -A INPUT -p tcp --dport 8082 -s 10.0.0.0/8 -j ACCEPT

# Block all other access to webhook/admin
iptables -A INPUT -p tcp --dport 8081 -j DROP
iptables -A INPUT -p tcp --dport 8082 -j DROP
```

### 3. Reverse Proxy Path Segregation

When using a reverse proxy (nginx, Traefik, Caddy), use different subdomains or paths:

**Option A: Subdomain Separation**
```nginx
# docs.example.com → Docs Server
server {
    server_name docs.example.com;
    location / {
        proxy_pass http://localhost:8080;
    }
}

# webhooks.example.com → Webhook Server
server {
    server_name webhooks.example.com;
    location / {
        proxy_pass http://localhost:8081;
    }
}

# admin.example.com → Admin Server (internal only)
server {
    server_name admin.example.com;
    allow 10.0.0.0/8;
    deny all;
    location / {
        proxy_pass http://localhost:8082;
    }
}
```

**Option B: Path Separation** (Less Recommended)
```nginx
server {
    server_name example.com;
    
    # Documentation at root
    location / {
        proxy_pass http://localhost:8080;
    }
    
    # Webhooks at /api/webhooks
    location /api/webhooks/ {
        proxy_pass http://localhost:8081/webhooks/;
    }
    
    # Admin at /api/admin
    location /api/admin/ {
        allow 10.0.0.0/8;
        deny all;
        proxy_pass http://localhost:8082/api/;
    }
}
```

### 4. Content Security Policy

For defense in depth, the docs server could set CSP headers to prevent accidental form submissions to webhook paths:

```http
Content-Security-Policy: form-action 'self'; frame-ancestors 'none'
```

This prevents JavaScript on the docs site from submitting forms to webhook endpoints.

## Attack Vectors (and Why They're Mitigated)

### ERROR: Path Traversal Attack
**Scenario**: Attacker tries `GET /webhooks/../../../etc/passwd`

**Mitigation**: 
- HTTP path normalization happens before routing
- Webhook server only handles `/webhooks/*`, not arbitrary paths
- Different port means request wouldn't reach docs server anyway

### ERROR: Documentation Collision
**Scenario**: Hugo generates a page at `/webhooks/github.html`

**Mitigation**:
- Webhook server is on port 8081, docs on 8080
- Even if page exists on docs server, webhook POST goes to webhook server
- HTTP method differs (GET vs POST)

### ERROR: Port Confusion
**Scenario**: User configures same port for docs and webhooks

**Mitigation**:
- Startup validation fails with clear error
- Daemon refuses to start
- Operator must fix configuration

### ERROR: Webhook Forgery via Docs
**Scenario**: Attacker embeds JavaScript in docs to forge webhooks

**Mitigation**:
- Different origins (port 8080 vs 8081) trigger CORS
- Webhook signature validation prevents unsigned requests
- Same-origin policy blocks cross-port requests

## Testing Collision Prevention

### Manual Testing

```bash
# Start daemon
./docbuilder daemon

# Verify docs server responds
curl http://localhost:8080/
# Expected: 200 OK (documentation)

# Verify webhook server responds (POST only)
curl http://localhost:8081/webhooks/github
# Expected: 405 Method Not Allowed

curl -X POST http://localhost:8081/webhooks/github
# Expected: 400 Bad Request (no payload) or 401 (no signature)

# Verify docs server doesn't handle webhooks
curl -X POST http://localhost:8080/webhooks/github
# Expected: 404 Not Found (no such documentation page)
```

### Port Conflict Testing

```bash
# Occupy port 8081
nc -l 8081 &

# Try to start DocBuilder
./docbuilder daemon
# Expected: Error: "webhook port 8081: address already in use"
```

## Configuration Best Practices

### - Recommended: Default Ports

```yaml
daemon:
  http:
    docs_port: 8080       # Standard HTTP alternative port
    webhook_port: 8081    # Sequential, clearly webhook-related
    admin_port: 8082      # Sequential, clearly admin-related
    livereload_port: 8083 # Sequential, optional feature
```

### - Recommended: Custom Ports with Separation

```yaml
daemon:
  http:
    docs_port: 3000       # Custom docs port
    webhook_port: 3001    # Different from docs
    admin_port: 3002      # Different from both
    livereload_port: 3003 # Different from all
```

### ERROR: Never: Same Ports

```yaml
daemon:
  http:
    docs_port: 8080
    webhook_port: 8080    # ERROR: WILL FAIL TO START
    admin_port: 8080      # ERROR: WILL FAIL TO START
```

### WARNING: Caution: Non-Sequential Ports

```yaml
daemon:
  http:
    docs_port: 8080
    webhook_port: 9443    # WARNING: Works but non-obvious relationship
    admin_port: 3000      # WARNING: Works but confusing
```

## Monitoring and Validation

### Startup Validation

Watch daemon logs for port binding confirmation:

```
INFO HTTP servers binding to ports docs_port=8080 webhook_port=8081 admin_port=8082
INFO Documentation server started on :8080
INFO Webhook server started on :8081
INFO Admin server started on :8082
```

If you see errors:

```
ERROR http startup failed: webhook port 8081: address already in use
```

This indicates a port conflict that must be resolved before the daemon can start.

### Runtime Health Checks

```bash
# Check all servers are responding
curl -f http://localhost:8080/health  # Docs server
curl -f http://localhost:8082/health  # Admin server

# Webhook server doesn't have health endpoint (POST-only)
# Use netstat instead:
netstat -an | grep :8081
# Expected: LISTEN state
```

## Summary

DocBuilder prevents webhook/documentation collisions through:

1. - **Port Isolation** - Separate HTTP servers on different ports (primary defense)
2. - **Path Prefixing** - Reserved `/webhooks/*` prefix (secondary defense)
3. - **Method Filtering** - POST-only webhooks vs GET documentation (tertiary defense)
4. - **Startup Validation** - Fail fast if ports conflict (preventive)
5. - **Firewall Rules** - Network-level access control (optional)
6. - **Reverse Proxy** - Subdomain/path segregation (optional)

**Collision Risk**: Effectively 0% with default configuration.

## Related Documentation

- [Configure Webhooks](../how-to/configure-webhooks.md) - Webhook setup guide
- [Getting Started](../tutorials/getting-started.md) - Introduction to DocBuilder
