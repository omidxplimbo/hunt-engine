# Hunt Engine / HuntOS
# نقشه راه جدید - مسیر هک واقعی
# New Roadmap - Real Bug Hunting Path
# v3.17.0 → v4.0.0

---

## قانون حاکم

این سند منبع حقیقت پروژه است. هر تغییر باید اول در این سند اعمال شود.
Latest roadmap = source of truth. No hidden roadmap.

---

## وضعیت فعلی: v3.16.0

### ✅ چیزی که داریم (Complete)

| Component | Status | Description |
|-----------|--------|-------------|
| Recon Pipeline | ✅ | subfinder, httpx, katana, nuclei, amass, puredns, dnsx, alterx |
| Findings System | ✅ | Create, status, evidence, export |
| AI Agents | ✅ | Triage, Summary, Report (deterministic) |
| Agent Chat | ✅ | Conversational operator with LLM |
| Operator Skills | ✅ | 16 built-in + user-defined skills |
| Operator Learning | ✅ | Methodology records, target skill profile |
| Auth Context | ✅ | Cookie/header/token/session storage |
| Two-Account Testing | ✅ | IDOR/BOLA basic testing |
| Target Memory | ✅ | RAG-style memory with chunking |
| Controlled Runtime | ✅ | HTTP probe, OWASP checklist |
| Bug Tests | ✅ | Pattern/payload registry |
| PDF Reports | ✅ | Basic report generation |
| Documentation | ✅ | Bilingual portal (FA/EN) |

### ⚠️ چیزی که ناقصه (Partial)

| Component | Status | Gap |
|-----------|--------|-----|
| Auto-Hunting Loop | ⚠️ | Manual trigger only, no auto-pipeline |
| Hypothesis Engine | ⚠️ | Basic, no prioritization |
| Evidence Scoring | ⚠️ | No auto-promotion to findings |
| Bug Chain Detection | ❌ | Not implemented |
| SQLi/SSRF/RCE | ❌ | Not in skill registry |
| Bug Bounty Report Format | ❌ | Generic PDF only |
| UI Simplification | ❌ | Analysis tab too crowded |

---

## نقشه راه جدید: مسیر هک واقعی

### هدف نهایی
تبدیل Hunt Engine به یک دستیار هک واقعی که بتواند:
1. بدون دخالت کاربر، باگ پیدا کند
2. باگ‌های پیدا شده را اولویت‌بندی کند
3. گزارش استاندارد Bug Bounty تولید کند
4. از نتایج یاد بگیرد و استراتژی خود را بهبود دهد

---

## v3.17.0 - Auto-Hunting Loop + UI Simplification

### هدف
ساده‌سازی تب Analysis و ساخت حلقه شکار خودکار

### خروجی قابل تست

| Task | Description | Test |
|------|-------------|------|
| Hunting Dashboard | تب Analysis جدید با 4 بخش اصلی | UI renders correctly |
| Auto-Hunt Worker | Background worker که بعد از ریکان شروع به تست می‌کند | New assets auto-analyzed |
| Hypothesis Prioritizer | اولویت‌بندی فرضیه‌ها بر اساس احتمال موفقیت | Top hypothesis suggested |
| Evidence Scorer | امتیازدهی خودکار به evidence | Score > 0.7 auto-promoted |
| Skill Auto-Selector | انتخاب خودکار مهارت مناسب بر اساس asset type | Correct skill selected |

### معماری جدید تب Analysis

```
┌─────────────────────────────────────────────────────────────┐
│                    🎯 HUNTING DASHBOARD                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Assets: 150  │  │ Findings: 12 │  │ Bugs: 3      │      │
│  │ Live: 89     │  │ Open: 8      │  │ Reportable: 2│      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  🤖 AGENT PIPELINE STATUS                           │   │
│  │  [Recon] → [Analysis] → [Exploit] → [Report]        │   │
│  │    ✅        🔄          ⏸️          ⏸️              │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  📋 TOP ACTIONS (AI-sorted)                         │   │
│  │  1. 🔴 Test IDOR on /api/users/{id} (Score: 92)    │   │
│  │  2. 🟡 Check XSS on search param (Score: 78)       │   │
│  │  3. 🟢 Validate SQLi on login (Score: 65)          │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  💬 QUICK CHAT                                      │   │
│  │  [Simplified chat for manual override]              │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### فنی

```go
// Auto-Hunt Worker
// بعد از اتمام ریکان، خودکار شروع به کار می‌کند
type AutoHuntWorker struct {
    TargetID    uint
    Phase       string // "analyzing", "hypothesizing", "testing", "reporting"
    Hypotheses  []Hypothesis
    Results     []TestResult
}

type Hypothesis struct {
    BugClass    string  // "idor", "xss", "sqli", etc.
    Endpoint    string
    Parameter   string
    Confidence  float64 // 0.0 - 1.0
    Priority    int     // 1-10
    SkillSlug   string  // مهارت مناسب برای تست
}
```

---

## v3.18.0 - High-Impact Bug Classes (SQLi/SSRF/RCE)

### هدف
اضافه کردن پرتکرارترین باگ‌های Bug Bounty

### خروجی قابل تست

| Task | Description | Test |
|------|-------------|------|
| SQLi Validator | تشخیص و اعتبارسنجی SQL Injection | Blind SQLi detected on param |
| SSRF Detector | تشخیص Server-Side Request Forgery | SSRF via URL parameter found |
| RCE Scanner | تشخیص Remote Code Execution | Command injection detected |
| SSTI Validator | تشخیص Server-Side Template Injection | Jinja2/Twig injection found |
| NoSQLi Tester | تشخیص NoSQL Injection | MongoDB operator injection |
| XXE Detector | تشخیص XML External Entity | XXE via file upload |

### مهارت‌های جدید

```yaml
# SQLi Validation Skill
name: SQL Injection Validator
slug: sqli_validation
category: injection
bug_class: sqli
trigger_signals: ["id", "user", "search", "filter", "sort", "page"]
runtime_backend: internal_http_runtime
safety_level: 2
test_level: 2

# مراحل تست:
# 1. تشخیص پارامترهای مستعد
# 2. تست با payload‌های blind SQLi
# 3. تشخیص response差异
# 4. تأیید با time-based blind
# 5. ذخیره evidence
```

### اولویت‌بندی باگ‌ها بر اساس Bug Bounty

| Priority | Bug Class | Avg Bounty | Detection Rate |
|----------|-----------|------------|----------------|
| 1 | IDOR/BOLA | $500-5000 | High |
| 2 | XSS (Reflected/Stored) | $150-3000 | High |
| 3 | SQL Injection | $500-10000 | Medium |
| 4 | SSRF | $500-5000 | Medium |
| 5 | RCE | $1000-20000 | Low |
| 6 | SSTI | $500-5000 | Low |
| 7 | Open Redirect | $150-1000 | High |
| 8 | CSRF | $150-1000 | Medium |

---

## v3.19.0 - Evidence Chain + Auto-Promotion

### هدف
تبدیل evidence به finding قابل گزارش

### خروجی قابل تست

| Task | Description | Test |
|------|-------------|------|
| Evidence Scorer | امتیازدهی خودکار به evidence | Score calculated correctly |
| Auto-Promotion | تبدیل خودکار evidence به finding | High-score evidence promoted |
| Bug Chain Detector | تشخیص زنجیره باگ‌ها | Chain of 2+ bugs detected |
| Impact Calculator | محاسبه impact واقعی | CVSS-like scoring |
| False Positive Filter | فیلتر کردن false positive‌ها | FP rate < 10% |

### فلوی Evidence → Finding

```
Evidence Captured
    ↓
Score Calculation (0-100)
    ↓
┌─────────────────────────────────────┐
│ Score < 40  → Discard (noise)       │
│ Score 40-70 → Keep as observation   │
│ Score 70-85 → Flag for review       │
│ Score > 85  → Auto-promote to Bug   │
└─────────────────────────────────────┘
    ↓
Finding Created
    ↓
Report Draft Generated
```

### معیارهای امتیازدهی

```go
type EvidenceScore struct {
    TechnicalSeverity   int // 0-25 (CVSS-like)
    Exploitability      int // 0-25 (how easy to exploit)
    Impact              int // 0-25 (business impact)
    Confidence          int // 0-25 (certainty)
    // Total: 0-100
}
```

---

## v3.20.0 - Bug Bounty Report Generator

### هدف
تولید گزارش استاندارد برای HackerOne, Bugcrowd, etc.

### خروجی قابل تست

| Task | Description | Test |
|------|-------------|------|
| Report Template | قالب استاندارد Bug Bounty | Template renders correctly |
| Auto-Sections | تولید خودکار بخش‌های گزارش | All sections populated |
| PoC Generator | تولید Proof of Concept | PoC is reproducible |
| Remediation Suggester | پیشنهاد رفع آسیب‌پذیری | Fix suggestions provided |
| Export Formats | خروجی Markdown, HTML, PDF | All formats work |

### قالب گزارش استاندارد

```markdown
# Title: [Bug Class] on [Endpoint]

## Summary
[1-2 sentence description]

## Severity
[Critical/High/Medium/Low/Info]

## Affected Endpoint
- URL: [endpoint]
- Method: [GET/POST/etc.]
- Parameters: [affected params]

## Steps to Reproduce
1. [Step 1]
2. [Step 2]
3. [Step 3]

## Proof of Concept
[Request/Response with evidence]

## Impact
[Business impact description]

## Remediation
[How to fix]

## References
[CVE/CWE links]
```

---

## v4.0.0 - True AI Pentest Operator

### هدف
جلسات خودمختار تست نفوذ با نظارت انسان

### خروجی قابل تست

| Task | Description | Test |
|------|-------------|------|
| Session Manager | مدیریت جلسات شکار | Session starts/stops correctly |
| Live Output | خروجی زنده از فرآیند شکار | Real-time updates in UI |
| Kill Switch | توقف فوری در صورت نیاز | Emergency stop works |
| Budget Manager | مدیریت بودجه درخواست‌ها | Budget limits enforced |
| Chain Exploiter | اکسپلویت زنجیره‌ای | Multi-step exploit works |
| Report Finalizer | نهایی‌سازی گزارش | Final report generated |

### معماری جلسه شکار

```
┌─────────────────────────────────────────────────────────────┐
│                    🎯 HUNTING SESSION                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Target: example.com                                        │
│  Objective: Find IDOR/XSS vulnerabilities                   │
│  Budget: 1000 requests remaining                            │
│  Duration: 45 minutes                                       │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  📊 LIVE PROGRESS                                   │   │
│  │  Phase: Exploiting                                   │   │
│  │  Hypotheses tested: 12/20                           │   │
│  │  Bugs found: 2 (1 High, 1 Medium)                   │   │
│  │  Current: Testing XSS on search parameter           │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  🔴 KILL SWITCH                                     │   │
│  │  [STOP NOW] - Immediately halt all testing          │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## خلاصه نسخه‌ها

| Version | Title | Key Output | Priority |
|---------|-------|------------|----------|
| v3.17.0 | Auto-Hunting Loop + UI | Hunting dashboard, auto-hunt worker | 🔴 Critical |
| v3.18.0 | High-Impact Bugs | SQLi/SSRF/RCE/SSTI/NoSQLi/XXE | 🔴 Critical |
| v3.19.0 | Evidence Chain | Auto-promotion, bug chains, scoring | 🟡 Important |
| v3.20.0 | Bug Bounty Reports | Standard format, PoC, remediation | 🟡 Important |
| v4.0.0 | True AI Operator | Autonomous sessions, chains, reports | 🟢 Vision |

---

## اولویت اجرایی فوری

### Phase 1: ساده‌سازی (v3.17.0) - 2 هفته
1. بازطراحی تب Analysis به Hunting Dashboard
2. ساخت Auto-Hunt Worker
3. پیاده‌سازی Hypothesis Prioritizer
4. اضافه کردن Evidence Scorer

### Phase 2: باگ‌های پرتکرار (v3.18.0) - 3 هفته
1. اضافه کردن SQLi Validator
2. اضافه کردن SSRF Detector
3. اضافه کردن RCE Scanner
4. اضافه کردن SSTI/NoSQLi/XXE

### Phase 3: کیفیت یافته‌ها (v3.19.0) - 2 هفته
1. پیاده‌سازی Evidence Scoring
2. ساخت Auto-Promotion system
3. اضافه کردن Bug Chain detection
4. بهبود False Positive filtering

### Phase 4: گزارش‌دهی (v3.20.0) - 2 هفته
1. طراحی قالب Bug Bounty
2. ساخت PoC Generator
3. اضافه کردن Remediation suggestions
4. خروجی Markdown/HTML/PDF

### Phase 5: اپراتور واقعی (v4.0.0) - 4 هفته
1. ساخت Session Manager
2. اضافه کردن Live Output
3. پیاده‌سازی Kill Switch
4. ساخت Chain Exploiter

---

## معیارهای موفقیت

### کوتاه‌مدت (1 ماه)
- [ ] تب Analysis ساده شده و 4 بخش اصلی دارد
- [ ] حلقه شکار خودکار کار می‌کند
- [ ] SQLi/SSRF اضافه شده‌اند

### میان‌مدت (3 ماه)
- [ ] حداقل 1 باگ واقعی در Bug Bounty پیدا شده
- [ ] گزارش استاندارد تولید می‌شود
- [ ] Evidence auto-promotion کار می‌کند

### بلندمدت (6 ماه)
- [ ] جلسات خودمختار شکار کار می‌کنند
- [ ] زنجیره باگ‌ها تشخیص داده می‌شوند
- [ ] گزارش‌های آماده ارسال به پلتفرم‌ها تولید می‌شوند

---

## پایان سند

این نقشه راه مسیر تبدیل Hunt Engine از یک ابزار ریکان به یک دستیار هک واقعی را مشخص می‌کند.
هر نسخه باید خروجی قابل تست داشته باشد و مستندات آن به‌روز شود.

**قانون: هر feature جدید باید با smoke test و documentation همراه باشد.**
