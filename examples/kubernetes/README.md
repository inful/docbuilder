# Kubernetes Deployment for DocBuilder

This directory contains Kubernetes manifests for deploying DocBuilder as a service in a Kubernetes cluster.

## Components

- **Namespace**: Isolated namespace for DocBuilder resources
- **ConfigMap**: Configuration file for DocBuilder
- **Secret**: Sensitive credentials (Git tokens)
- **PersistentVolumeClaim**: Storage for generated documentation
- **Deployment**: DocBuilder application deployment
- **Service**: Internal service for accessing DocBuilder
- **Ingress**: (Optional) External access configuration
- **ServiceMonitor**: (Optional) Prometheus monitoring integration

## Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured to access your cluster
- (Optional) Ingress controller (nginx, traefik, etc.)
- (Optional) cert-manager for TLS certificates
- (Optional) Prometheus Operator for monitoring

## Quick Start

### 1. Update Configuration

Edit `deployment.yaml` to customize:

```yaml
# Update the ConfigMap with your repositories
repositories:
  - url: https://github.com/your-org/your-repo.git
    name: your-docs
    branch: main
    paths: ["docs"]

# Update the Secret with your actual tokens
stringData:
  GITHUB_TOKEN: "your-actual-github-token"
```

### 2. Deploy to Kubernetes

```bash
# Deploy all resources
kubectl apply -f deployment.yaml

# Check deployment status
kubectl get all -n docbuilder

# View logs
kubectl logs -n docbuilder -l app=docbuilder -f

# Check config
kubectl get configmap -n docbuilder docbuilder-config -o yaml
```

### 3. Access the Service

#### Inside the Cluster

```bash
# Port forward to access locally
kubectl port-forward -n docbuilder svc/docbuilder 8080:8080

# Access at http://localhost:8080
```

#### External Access (with Ingress)

Update the Ingress section with your domain:

```yaml
spec:
  rules:
  - host: docs.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: docbuilder
            port:
              number: 8080
```

Then apply and access via your domain.

## Configuration Options

### Environment Variables

Add environment variables to the Deployment:

```yaml
env:
- name: LOG_LEVEL
  value: "debug"
- name: CUSTOM_VAR
  value: "value"
```

### Resource Limits

Adjust resource requests/limits based on your workload:

```yaml
resources:
  requests:
    cpu: 500m      # Increase for faster builds
    memory: 512Mi  # Increase for large repos
  limits:
    cpu: 2000m
    memory: 2Gi
```

### Storage

Adjust PVC size based on output needs:

```yaml
resources:
  requests:
    storage: 10Gi  # Increase for more documentation
```

### Multiple Replicas

For high availability (requires ReadWriteMany PVC or shared storage):

```yaml
spec:
  replicas: 2
```

## Using Private Container Registry

If your DocBuilder image is in a private registry:

### 1. Create Registry Secret

```bash
kubectl create secret docker-registry registry-credentials \
  --namespace=docbuilder \
  --docker-server=git.home.luguber.info \
  --docker-username=your-username \
  --docker-password=your-token \
  --docker-email=your-email@example.com
```

### 2. Reference in Deployment

```yaml
spec:
  imagePullSecrets:
  - name: registry-credentials
```

## Monitoring

### Health Checks

DocBuilder exposes health endpoints:

- `/health` - Liveness probe
- `/ready` - Readiness probe

### Metrics

Prometheus metrics available at:

- Port 9090: `/metrics`

### Enable ServiceMonitor

If using Prometheus Operator:

```bash
# ServiceMonitor is already included in deployment.yaml
# Just ensure prometheus-operator is installed
kubectl get servicemonitor -n docbuilder
```

## TLS/HTTPS Setup

### Using cert-manager

1. Install cert-manager:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

2. Create ClusterIssuer:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

3. Update Ingress annotations:

```yaml
metadata:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - docs.yourdomain.com
    secretName: docbuilder-tls
```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n docbuilder
kubectl describe pod -n docbuilder <pod-name>
```

### View Logs

```bash
# All logs
kubectl logs -n docbuilder -l app=docbuilder

# Follow logs
kubectl logs -n docbuilder -l app=docbuilder -f

# Previous container logs (if crashed)
kubectl logs -n docbuilder <pod-name> --previous
```

### Check ConfigMap

```bash
kubectl get configmap -n docbuilder docbuilder-config -o yaml
```

### Check Secrets

```bash
kubectl get secret -n docbuilder docbuilder-secrets -o yaml
```

### Exec into Pod

```bash
kubectl exec -it -n docbuilder <pod-name> -- sh
```

### Check Service Connectivity

```bash
# Test from another pod
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://docbuilder.docbuilder.svc.cluster.local:8080/health
```

## Updating Configuration

### Update ConfigMap

```bash
# Edit the configmap
kubectl edit configmap -n docbuilder docbuilder-config

# Or apply updated YAML
kubectl apply -f deployment.yaml

# Restart pods to pick up changes
kubectl rollout restart deployment -n docbuilder docbuilder
```

### Update Secrets

```bash
# Update via kubectl
kubectl create secret generic docbuilder-secrets \
  --from-literal=GITHUB_TOKEN=new-token \
  --namespace=docbuilder \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart deployment
kubectl rollout restart deployment -n docbuilder docbuilder
```

## Cleanup

```bash
# Delete all resources
kubectl delete -f deployment.yaml

# Or delete namespace (removes everything)
kubectl delete namespace docbuilder
```

## Production Best Practices

1. **Use External Secrets**: Consider using External Secrets Operator or Sealed Secrets for managing sensitive data
2. **Resource Limits**: Set appropriate CPU/memory limits based on your workload
3. **Monitoring**: Enable Prometheus monitoring and set up alerts
4. **Backups**: Configure regular backups of the PVC
5. **RBAC**: Apply proper RBAC policies for service accounts
6. **Network Policies**: Restrict pod-to-pod communication
7. **Pod Security**: Use Pod Security Standards/Policies
8. **High Availability**: Use multiple replicas with proper anti-affinity rules
9. **Autoscaling**: Consider HPA based on metrics
10. **GitOps**: Use ArgoCD or Flux for declarative deployments

## Advanced Configuration

### Using Kustomize

Create `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: docbuilder

resources:
- deployment.yaml

# Overlays for different environments
configMapGenerator:
- name: docbuilder-config
  files:
  - config.yaml

# Patches for environment-specific changes
patches:
- target:
    kind: Deployment
    name: docbuilder
  patch: |-
    - op: replace
      path: /spec/replicas
      value: 2
```

Deploy with:

```bash
kubectl apply -k .
```

## Examples

See the main `examples/` directory for:
- `configs/` - Sample configuration files
- `docker-compose.yml` - Docker Compose setup
- `systemd/` - Systemd service files
