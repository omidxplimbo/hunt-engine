---
name: idor_bola
description: IDOR/BOLA testing with horizontal and vertical privilege escalation
category: access_control
bug_class: idor
triggers:
  - idor
  - bola
  - access control
  - privilege escalation
  - authorization
---

# IDOR/BOLA Testing

## Key Insight
IDOR (Insecure Direct Object Reference) occurs when an application uses user-supplied input to access objects directly without authorization checks. BOLA (Broken Object Level Authorization) is the API equivalent.

## Attack Surface
- URL parameters (id, user_id, account_id, order_id)
- API endpoints with resource identifiers
- File download/upload endpoints
- Profile/account management endpoints
- Admin panels accessible by regular users

## Methodology
1. **Identify Object References**: Find all endpoints that use IDs
2. **Map User Roles**: Understand the permission model
3. **Horizontal Testing**: Access other users' resources at same privilege level
4. **Vertical Testing**: Access higher-privilege resources
5. **Document Evidence**: Capture the exact request/response difference

## Techniques

### Horizontal Privilege Escalation
```
# Change user_id in request
GET /api/users/123/profile  (your user)
GET /api/users/456/profile  (other user)

# Change account_id
GET /api/accounts/123/orders
GET /api/accounts/456/orders
```

### Vertical Privilege Escalation
```
# Access admin endpoints as regular user
GET /api/admin/users
GET /api/admin/settings

# Modify role in request
POST /api/users/123/update
{"role": "admin"}
```

### UUID/Sequential ID Testing
```
# Sequential IDs
GET /api/documents/1
GET /api/documents/2
GET /api/documents/3

# UUID enumeration
GET /api/documents/550e8400-e29b-41d4-a716-446655440000
```

## Validation
- Compare responses between two accounts
- Check if sensitive data is returned
- Verify the data belongs to the other user
- Test both read and write operations

## False Positive Avoidance
- Public data is NOT IDOR
- Properly authorized access is NOT IDOR
- Rate-limited endpoints may return different responses
