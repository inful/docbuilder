# VS Code Edit Handler Security

## Overview

The VS Code edit handler (`/_edit/` endpoint) allows opening documentation files directly in VS Code from the preview server. Since it executes local commands, it implements multiple security layers.

## Security Model

### Threat Model

**Potential Attackers:**
- Malicious HTTP requests from unauthorized users
- Path traversal attacks to access files outside docs directory  
- Command injection via file paths or IPC socket paths
- Symlink attacks to access sensitive system files
- Environment variable injection via socket paths

**Assets Protected:**
- Local file system (read access via VS Code)
- Command execution (VS Code CLI)
- Environment variables

## Security Controls

### 1. Feature Flag Protection
```go
if !s.config.Build.VSCodeEditLinks {
    return http.StatusNotFound
}
```
**Control:** Feature disabled by default, requires explicit `--vscode` flag  
**Threat Mitigated:** Unauthorized access  
**OWASP:** A01:2021 Broken Access Control

### 2. Mode Restriction
```go
if s.config.Daemon != nil && s.config.Daemon.Storage.RepoCacheDir != "" {
    return http.StatusNotImplemented
}
```
**Control:** Only works in preview mode (single local repository)  
**Threat Mitigated:** Multi-repository security boundary violations  
**OWASP:** A01:2021 Broken Access Control

### 3. Path Traversal Protection
```go
cleanDocs := filepath.Clean(docsDir)
cleanPath := filepath.Clean(absPath)
if !strings.HasPrefix(cleanPath, cleanDocs) {
    return errors.New("Invalid file path")
}
```
**Control:** Validates resolved paths stay within docs directory  
**Threat Mitigated:** Path traversal (../../../etc/passwd)  
**OWASP:** A01:2021 Broken Access Control

**Additional Hardening:**
- Directory separator enforcement to prevent prefix confusion
- Absolute path requirement
- Clean path normalization

### 4. Symlink Attack Prevention
```go
fileInfo, err := os.Lstat(path)  // Don't follow symlinks
if fileInfo.Mode()&os.ModeSymlink != 0 {
    return errors.New("Symlinks are not allowed")
}
```
**Control:** Rejects all symlinks  
**Threat Mitigated:** Symlink attacks to escape docs directory  
**OWASP:** A01:2021 Broken Access Control  
**CVE Examples:** CVE-2022-24765 (Git symlink traversal)

### 5. File Type Restriction
```go
ext := strings.ToLower(filepath.Ext(path))
if ext != ".md" && ext != ".markdown" {
    return errors.New("Only markdown files can be edited")
}
```
**Control:** Only markdown files allowed  
**Threat Mitigated:** Opening sensitive non-documentation files  
**OWASP:** A01:2021 Broken Access Control

### 6. Command Injection Prevention
```go
// Direct exec without shell
cmd := exec.CommandContext(ctx, codeCmd, "--reuse-window", "--goto", absPath)
```
**Control:** No shell invocation (`bash -c`), direct exec only  
**Threat Mitigated:** Shell injection attacks  
**OWASP:** A03:2021 Injection  
**CWE:** CWE-78 (OS Command Injection)

**Previous vulnerable pattern:**
```go
// INSECURE - Don't do this
cmd := exec.Command("bash", "-c", "code --goto "+path)
```

### 7. IPC Socket Path Validation
```go
func validateIPCSocketPath(socketPath string) error {
    // Reject control characters
    if strings.ContainsAny(socketPath, "\n\r\x00") {
        return errors.New("invalid characters")
    }
    // Require absolute path
    if !filepath.IsAbs(socketPath) {
        return errors.New("must be absolute")
    }
    // Verify expected location
    if !strings.HasPrefix(socketPath, "/tmp/vscode-ipc-") { ... }
    // Require .sock extension
    if !strings.HasSuffix(socketPath, ".sock") { ... }
}
```
**Control:** Multi-layered socket path validation  
**Threats Mitigated:**
- Environment variable injection via newlines
- Malicious socket paths
- Relative path attacks

### 8. Execution Timeout
```go
ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
defer cancel()
```
**Control:** 5 second timeout on command execution  
**Threat Mitigated:** Resource exhaustion, hanging processes  
**OWASP:** A05:2021 Security Misconfiguration

### 9. Validated Executable Paths
```go
vscodePaths := []string{
    "/vscode/vscode-server/bin/*/bin/remote-cli/code",
    "/usr/local/bin/code",
    "/usr/bin/code",
}
```
**Control:** Code CLI path from trusted locations only  
**Threat Mitigated:** Malicious executable substitution  
**OWASP:** A08:2021 Software and Data Integrity Failures

## Security Testing

### Test Coverage
- ✅ Feature flag enforcement
- ✅ Path traversal attempts
- ✅ Symlink detection and rejection
- ✅ IPC socket validation (injection attempts)
- ✅ File type restrictions
- ✅ Invalid paths and edge cases

### Penetration Testing Scenarios

**1. Path Traversal:**
```bash
curl http://localhost:1314/_edit/../../../etc/passwd
# Expected: 400 Bad Request
```

**2. Symlink Attack:**
```bash
ln -s /etc/passwd docs/evil.md
curl http://localhost:1314/_edit/evil.md
# Expected: 403 Forbidden
```

**3. Control Character Injection:**
```bash
# If socket path contained newlines (blocked by validation)
VSCODE_IPC_HOOK_CLI="/tmp/sock\nMALICIOUS=1"
# Expected: 500 Internal Server Error
```

**4. Feature Disabled:**
```bash
# Without --vscode flag
curl http://localhost:1314/_edit/test.md
# Expected: 404 Not Found
```

## Defense in Depth

Multiple security layers ensure that even if one control fails, others prevent exploitation:

1. **Feature Flag** → Prevents access by default
2. **Mode Check** → Limits to preview mode
3. **Path Validation** → Blocks directory traversal
4. **Symlink Check** → Prevents indirect traversal
5. **File Type** → Limits to markdown only
6. **Direct Exec** → No shell injection possible
7. **Socket Validation** → Prevents environment injection
8. **Timeout** → Limits resource usage

## Security Recommendations

### For Operators

1. **Only use `--vscode` flag in trusted development environments**
2. **Never expose preview server to untrusted networks**
3. **Use preview mode only with trusted repositories**
4. **Monitor logs for suspicious access attempts**

### For Developers

1. **Never remove security validations**
2. **Add tests for new security controls**
3. **Run security-focused linters (gosec)**
4. **Review shell command execution patterns**
5. **Validate all external inputs**

## Incident Response

If security issue is discovered:

1. **Disable feature immediately** (remove `--vscode` flag)
2. **Check logs** for exploitation attempts
3. **Report via security policy**
4. **Apply patch and test thoroughly**
5. **Update security documentation**

## References

- [OWASP Top 10 2021](https://owasp.org/www-project-top-ten/)
- [CWE-78: OS Command Injection](https://cwe.mitre.org/data/definitions/78.html)
- [CWE-22: Path Traversal](https://cwe.mitre.org/data/definitions/22.html)
- [Go Security Best Practices](https://go.dev/doc/security/best-practices)

## Changelog

- **2026-01-05**: Initial security review and hardening
  - Removed shell invocation (`bash -c`)
  - Added symlink detection
  - Added IPC socket path validation
  - Improved path traversal protection
