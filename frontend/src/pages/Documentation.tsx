import '@fontsource/vazirmatn/400.css';
import '@fontsource/vazirmatn/500.css';
import '@fontsource/vazirmatn/600.css';
import '@fontsource/vazirmatn/700.css';

import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  BookOpen,
  BrainCircuit,
  Database,
  FileCode2,
  Filter,
  Globe,
  LayoutDashboard,
  Lock,
  Search,
  Settings,
  Shield,
  TerminalSquare,
  User2,
} from 'lucide-react';
import clsx from 'clsx';
import MustacheLogo from '../components/MustacheLogo';

type DocsLang = 'fa' | 'en';

type DocsFeature = {
  id: string;
  group: string;
  icon: string;
  title: {
    fa: string;
    en: string;
  };
  route: string;
  purpose: {
    fa: string;
    en: string;
  };
  features: {
    fa: string[];
    en: string[];
  };
  howToUse: {
    fa: string[];
    en: string[];
  };
  outputs: {
    fa: string[];
    en: string[];
  };
  errors: {
    fa: string[];
    en: string[];
  };
  security: {
    fa: string[];
    en: string[];
  };
  screenshot: string;
};

const featureDocs: DocsFeature[] = [
  {
    id: 'landing-login',
    group: 'access',
    icon: 'lock',
    title: { fa: 'Landing، Login و ورود به سیستم', en: 'Landing, Login, and Access' },
    route: '/ و /login',
    purpose: {
      fa: 'نقطه ورود کاربر به Hunt Engine، نمایش صفحه عمومی، ورود امن، redirect کاربر authenticated به dashboard و مدیریت session.',
      en: 'The entry point for Hunt Engine: public landing, secure login, authenticated redirect to dashboard, and session handling.',
    },
    features: {
      fa: ['صفحه عمومی قبل از ورود', 'فرم login', 'ذخیره token در مرورگر', 'redirect خودکار کاربر authenticated', 'حذف token هنگام logout'],
      en: ['Public landing page', 'Login form', 'Browser token storage', 'Authenticated user redirect', 'Token removal on logout'],
    },
    howToUse: {
      fa: ['آدرس اصلی را باز کنید.', 'روی Initialize Session یا Login بروید.', 'username/password را وارد کنید.', 'بعد از ورود به Dashboard منتقل می‌شوید.'],
      en: ['Open the main URL.', 'Choose Initialize Session or Login.', 'Enter username/password.', 'After login, you are redirected to Dashboard.'],
    },
    outputs: {
      fa: ['Token فعال', 'username و role ذخیره‌شده', 'دسترسی به routeهای protected'],
      en: ['Active token', 'Stored username and role', 'Access to protected routes'],
    },
    errors: {
      fa: ['Invalid credentials', 'expired token', 'redirect loop', 'عدم دسترسی به route protected'],
      en: ['Invalid credentials', 'expired token', 'redirect loop', 'protected route denied'],
    },
    security: {
      fa: ['رمز و token را در screenshot مستندات نمایش ندهید.', 'خطای 401 فقط وقتی token نامعتبر است logout کند.'],
      en: ['Never expose passwords or tokens in documentation screenshots.', '401 should trigger logout only for invalid token cases.'],
    },
    screenshot: '/docs/screenshots/fa/access-login.png',
  },
  {
    id: 'dashboard',
    group: 'dashboard',
    icon: 'dashboard',
    title: { fa: 'Dashboard / Command Center', en: 'Dashboard / Command Center' },
    route: '/dashboard',
    purpose: {
      fa: 'نمای کلی وضعیت پروژه شامل تعداد targetها، assetها، live nodes، fresh intel و وضعیت runtime/processها.',
      en: 'A high-level project overview: targets, assets, live nodes, fresh intel, and runtime/process status.',
    },
    features: {
      fa: ['Total Targets', 'Total Assets', 'Live Nodes', 'Fresh Intel 24h', 'Active process overview', 'Top technologies/open ports'],
      en: ['Total Targets', 'Total Assets', 'Live Nodes', 'Fresh Intel 24h', 'Active process overview', 'Top technologies/open ports'],
    },
    howToUse: {
      fa: ['بعد از login وارد Dashboard شوید.', 'آمار کلی را بررسی کنید.', 'اگر process فعال دارید وضعیت runtime را چک کنید.', 'برای شروع کار به Targets بروید.'],
      en: ['Open Dashboard after login.', 'Review global stats.', 'Check active runtime/process indicators.', 'Move to Targets to start work.'],
    },
    outputs: {
      fa: ['آمار کلی platform', 'نمای سریع health پروژه', 'اولویت targetهایی که نیاز به بررسی دارند'],
      en: ['Global platform stats', 'Fast project health view', 'Targets that need review'],
    },
    errors: {
      fa: ['آمار صفر به دلیل نبود target', 'خطای API stats', 'عدم نمایش process به دلیل نبود job فعال'],
      en: ['Zero stats when no targets exist', 'Stats API error', 'No process display when no jobs are active'],
    },
    security: {
      fa: ['Dashboard فقط summary نشان می‌دهد؛ برای evidence واقعی وارد target شوید.'],
      en: ['Dashboard is summary-only; open a target for real evidence.'],
    },
    screenshot: '/docs/screenshots/fa/dashboard-command-center.png',
  },
  {
    id: 'targets-list',
    group: 'targets',
    icon: 'globe',
    title: { fa: 'Targets List و مدیریت Targetها', en: 'Targets List and Target Management' },
    route: '/targets',
    purpose: {
      fa: 'مدیریت targetها، ساخت target جدید، import/export، start/stop/restart scan و ورود به صفحه جزئیات target.',
      en: 'Manage targets, create new targets, import/export, start/stop/restart scans, and open target details.',
    },
    features: {
      fa: ['Create Target', 'Import Target', 'Export Targets', 'Stop Scan', 'Fresh Restart', 'Resume/Execute', 'Edit Target', 'Delete Target'],
      en: ['Create Target', 'Import Target', 'Export Targets', 'Stop Scan', 'Fresh Restart', 'Resume/Execute', 'Edit Target', 'Delete Target'],
    },
    howToUse: {
      fa: ['روی New Target کلیک کنید.', 'نام، root domain، description و modules را تنظیم کنید.', 'برای اجرای recon از execute/resume استفاده کنید.', 'برای بررسی نتایج روی نام target کلیک کنید.'],
      en: ['Click New Target.', 'Set name, root domain, description, and modules.', 'Use execute/resume for recon.', 'Open the target name to review results.'],
    },
    outputs: {
      fa: ['Target جدید', 'scan job فعال', 'asset/url/findingهای مربوط به target', 'export JSON'],
      en: ['New target', 'active scan job', 'target assets/URLs/findings', 'export JSON'],
    },
    errors: {
      fa: ['domain نامعتبر', 'target duplicate', 'scan already running', 'import JSON invalid'],
      en: ['Invalid domain', 'duplicate target', 'scan already running', 'invalid import JSON'],
    },
    security: {
      fa: ['قبل از اجرای scan مطمئن شوید root domain داخل scope مجاز است.'],
      en: ['Before running scans, confirm the root domain is authorized and in scope.'],
    },
    screenshot: '/docs/screenshots/fa/targets-list.png',
  },
  {
    id: 'target-detail-assets',
    group: 'target-detail',
    icon: 'database',
    title: { fa: 'Target Detail / Assets Tab', en: 'Target Detail / Assets Tab' },
    route: '/targets/:id → Assets',
    purpose: {
      fa: 'نمای اصلی attack surface شامل subdomainها، وضعیت live/dead، IP، ports، CDN، WAF، cloud و stack.',
      en: 'The main attack-surface table: subdomains, live/dead status, IPs, ports, CDN, WAF, cloud, and stack.',
    },
    features: {
      fa: ['Search assets', 'All/Live/Dead', 'Web', 'DNS', 'Ports', 'No CDN/CDN', 'WAF', 'Cloud', 'Provider filter', 'Status filter', 'Export IPs', 'Export Assets', 'PDF Report'],
      en: ['Search assets', 'All/Live/Dead', 'Web', 'DNS', 'Ports', 'No CDN/CDN', 'WAF', 'Cloud', 'Provider filter', 'Status filter', 'Export IPs', 'Export Assets', 'PDF Report'],
    },
    howToUse: {
      fa: ['Target را باز کنید.', 'Assets tab را انتخاب کنید.', 'با فیلترها سطح حمله را narrow کنید.', 'live web assets را برای تحلیل بعدی اولویت بدهید.', 'در صورت نیاز export بگیرید.'],
      en: ['Open a target.', 'Select Assets tab.', 'Use filters to narrow attack surface.', 'Prioritize live web assets for follow-up.', 'Export when needed.'],
    },
    outputs: {
      fa: ['لیست assetها', 'source/provider attribution', 'status live/dead', 'HTTP/DNS/port/CDN/WAF/cloud evidence'],
      en: ['Asset list', 'source/provider attribution', 'live/dead status', 'HTTP/DNS/port/CDN/WAF/cloud evidence'],
    },
    errors: {
      fa: ['فیلتر خالی', 'pagination اشتباه', 'asset dead به دلیل DNS/WAF/network', 'عدم وجود A record'],
      en: ['Empty filter result', 'pagination mismatch', 'dead asset due to DNS/WAF/network', 'missing A record'],
    },
    security: {
      fa: ['dead یا blocked را vulnerability حساب نکنید؛ evidence را بررسی کنید.'],
      en: ['Do not treat dead or blocked assets as vulnerabilities; review evidence.'],
    },
    screenshot: '/docs/screenshots/fa/target-assets-tab.png',
  },
  {
    id: 'target-urls',
    group: 'target-detail',
    icon: 'search',
    title: { fa: 'Intel / URLs و URL Inventory', en: 'Intel / URLs and URL Inventory' },
    route: '/targets/:id → Intel / URLs',
    purpose: {
      fa: 'بررسی URLهای جمع‌آوری‌شده از wayback، gau، katana، waymore، VirusTotal و سایر sourceها.',
      en: 'Review URLs collected from wayback, gau, katana, waymore, VirusTotal, and other sources.',
    },
    features: {
      fa: ['Search URLs', 'Only JS', 'Source filters', 'Sort by created/value/source', 'Export URLs', 'Resource Locator table'],
      en: ['Search URLs', 'Only JS', 'Source filters', 'Sort by created/value/source', 'Export URLs', 'Resource Locator table'],
    },
    howToUse: {
      fa: ['Intel / URLs را باز کنید.', 'برای endpointهای حساس search کنید.', 'only JS را برای JavaScript intelligence فعال کنید.', 'source را برای attribution فیلتر کنید.'],
      en: ['Open Intel / URLs.', 'Search for sensitive endpoints.', 'Enable only JS for JavaScript intelligence.', 'Filter by source for attribution.'],
    },
    outputs: {
      fa: ['URL inventory', 'JS candidates', 'source attribution', 'created time', 'export file'],
      en: ['URL inventory', 'JS candidates', 'source attribution', 'created time', 'export file'],
    },
    errors: {
      fa: ['URL تکراری', 'source خالی', 'JS کم به دلیل crawler config', 'export بدون نتیجه'],
      en: ['Duplicate URL', 'missing source', 'low JS count due to crawler config', 'empty export'],
    },
    security: {
      fa: ['URLهای خارج از scope را برای تست فعال استفاده نکنید.'],
      en: ['Do not actively test out-of-scope URLs.'],
    },
    screenshot: '/docs/screenshots/fa/target-urls-tab.png',
  },
  {
    id: 'target-policy',
    group: 'target-detail',
    icon: 'shield',
    title: { fa: 'Target Policy و Autopilot Controls', en: 'Target Policy and Autopilot Controls' },
    route: '/targets/:id → Policy',
    purpose: {
      fa: 'تعریف scope، mode operator، auto-execute levelها، approval برای تست‌های حساس، rate limit و execution boundaries.',
      en: 'Define scope, operator mode, auto-execute levels, approval for sensitive tests, rate limits, and execution boundaries.',
    },
    features: {
      fa: ['operator_mode', 'auto_execute_level_0', 'auto_execute_level_1', 'require_approval_level_2', 'require_approval_level_3', 'scope boundaries', 'rate limits'],
      en: ['operator_mode', 'auto_execute_level_0', 'auto_execute_level_1', 'require_approval_level_2', 'require_approval_level_3', 'scope boundaries', 'rate limits'],
    },
    howToUse: {
      fa: ['Policy tab را باز کنید.', 'mode را manual/assisted/strict تنظیم کنید.', 'سطوح auto execution و approval را مشخص کنید.', 'قبل از active validation policy را ذخیره کنید.'],
      en: ['Open Policy tab.', 'Set manual/assisted/strict mode.', 'Configure auto execution and approval levels.', 'Save policy before active validation.'],
    },
    outputs: {
      fa: ['policy ذخیره‌شده', 'رفتار operator در chat/runtime', 'approval requirement برای actionها'],
      en: ['Saved policy', 'Operator chat/runtime behavior', 'approval requirement for actions'],
    },
    errors: {
      fa: ['action به دلیل policy blocked', 'approval required', 'out-of-scope decision', 'rate/budget exceeded'],
      en: ['Action blocked by policy', 'approval required', 'out-of-scope decision', 'rate/budget exceeded'],
    },
    security: {
      fa: ['Policy مرز اجرای مجاز است؛ برای exploit واقعی باید authorization، budget، audit و stop condition مشخص باشد.'],
      en: ['Policy defines authorized execution; real exploit validation requires authorization, budget, audit, and stop conditions.'],
    },
    screenshot: '/docs/screenshots/fa/target-policy-tab.png',
  },
  {
    id: 'findings',
    group: 'target-detail',
    icon: 'shield',
    title: { fa: 'Findings و Evidence Review', en: 'Findings and Evidence Review' },
    route: '/targets/:id → Findings',
    purpose: {
      fa: 'بررسی findingها، severity، confidence، status، source_tool، evidence_json و triage status.',
      en: 'Review findings, severity, confidence, status, source_tool, evidence_json, and triage status.',
    },
    features: {
      fa: ['Finding filters', 'Severity', 'Status', 'Source tool', 'Search', 'Triage note', 'Export CSV/JSON'],
      en: ['Finding filters', 'Severity', 'Status', 'Source tool', 'Search', 'Triage note', 'Export CSV/JSON'],
    },
    howToUse: {
      fa: ['Findings tab را باز کنید.', 'severity/status را فیلتر کنید.', 'evidence_json را بررسی کنید.', 'status و triage note را فقط با evidence کافی تغییر دهید.'],
      en: ['Open Findings tab.', 'Filter severity/status.', 'Inspect evidence_json.', 'Change status and triage note only with enough evidence.'],
    },
    outputs: {
      fa: ['finding list', 'evidence_json', 'triage status', 'export findings'],
      en: ['finding list', 'evidence_json', 'triage status', 'export findings'],
    },
    errors: {
      fa: ['false positive', 'inconclusive evidence', 'blocked/WAF response', 'missing reproduction'],
      en: ['false positive', 'inconclusive evidence', 'blocked/WAF response', 'missing reproduction'],
    },
    security: {
      fa: ['403/429/5xx، WAF و blocked evidence را vulnerability قطعی claim نکنید.'],
      en: ['Do not claim 403/429/5xx, WAF, or blocked evidence as confirmed vulnerabilities.'],
    },
    screenshot: '/docs/screenshots/fa/findings-panel.png',
  },
  {
    id: 'operator-chat',
    group: 'operator',
    icon: 'brain',
    title: { fa: 'Attack Surface Chat / AI Operator', en: 'Attack Surface Chat / AI Operator' },
    route: '/targets/:id → Analysis → Attack Surface Chat',
    purpose: {
      fa: 'Operator شواهد target، memory، policy و skillها را ترکیب می‌کند تا hypothesis، next step و controlled validation بسازد.',
      en: 'The Operator combines target evidence, memory, policy, and skills to produce hypotheses, next steps, and controlled validation.',
    },
    features: {
      fa: ['Chat sessions', 'selected_skills', 'skill_execution', 'controlled runtime output', 'observations', 'memory learning', 'approval-required actions'],
      en: ['Chat sessions', 'selected_skills', 'skill_execution', 'controlled runtime output', 'observations', 'memory learning', 'approval-required actions'],
    },
    howToUse: {
      fa: ['از Operator سؤال دقیق بپرسید.', 'selected skills و runtime_scope را بررسی کنید.', 'برای action حساس approval بدهید یا رد کنید.', 'نتیجه evidence/learning را بررسی کنید.'],
      en: ['Ask precise questions.', 'Review selected skills and runtime_scope.', 'Approve or reject sensitive actions.', 'Review evidence/learning output.'],
    },
    outputs: {
      fa: ['hypotheses', 'skill_execution', 'output_json', 'observation_ids', 'next_step', 'blocked/inconclusive classification'],
      en: ['hypotheses', 'skill_execution', 'output_json', 'observation_ids', 'next_step', 'blocked/inconclusive classification'],
    },
    errors: {
      fa: ['not_implemented skill', 'policy blocked', 'missing auth context', 'inconclusive probe', 'no candidates'],
      en: ['not_implemented skill', 'policy blocked', 'missing auth context', 'inconclusive probe', 'no candidates'],
    },
    security: {
      fa: ['Operator باید scope-aware باشد؛ payload/state-changing/exploit action بدون authorization اجرا نشود.'],
      en: ['Operator must be scope-aware; payload/state-changing/exploit actions require authorization.'],
    },
    screenshot: '/docs/screenshots/fa/operator-chat.png',
  },
  {
    id: 'operator-skills',
    group: 'operator',
    icon: 'brain',
    title: { fa: 'Executable Skills و User-defined Skills', en: 'Executable Skills and User-defined Skills' },
    route: '/operator-skills',
    purpose: {
      fa: 'مدیریت skillهای اجرایی و قابل برنامه‌ریزی: built-in، user-defined، runtime backend، permission mode، budget و stop conditions.',
      en: 'Manage executable/plannable skills: built-in, user-defined, runtime backend, permission mode, budget, and stop conditions.',
    },
    features: {
      fa: ['Skill list', 'Include disabled', 'Create skill', 'Edit skill', 'Delete skill', 'runtime_backend', 'permission_mode', 'risk/safety/test/autonomy levels'],
      en: ['Skill list', 'Include disabled', 'Create skill', 'Edit skill', 'Delete skill', 'runtime_backend', 'permission_mode', 'risk/safety/test/autonomy levels'],
    },
    howToUse: {
      fa: ['Executable Skills را باز کنید.', 'skillها را search/filter کنید.', 'برای skill جدید metadata و permission را تنظیم کنید.', 'قبل از runtime واقعی budget و stop conditions را تعریف کنید.'],
      en: ['Open Executable Skills.', 'Search/filter skills.', 'Set metadata and permission for new skills.', 'Define budget and stop conditions before real runtime execution.'],
    },
    outputs: {
      fa: ['skill definition', 'runtime backend', 'execution profile', 'authorization requirements', 'budget defaults'],
      en: ['skill definition', 'runtime backend', 'execution profile', 'authorization requirements', 'budget defaults'],
    },
    errors: {
      fa: ['slug invalid/duplicate', 'runtime backend unsupported', 'permission mode نادرست', 'JSON metadata invalid'],
      en: ['invalid/duplicate slug', 'unsupported runtime backend', 'wrong permission mode', 'invalid JSON metadata'],
    },
    security: {
      fa: ['User-defined skillها در آینده می‌توانند execution واقعی داشته باشند؛ پس permission/budget/stop/audit باید مستند باشد.'],
      en: ['User-defined skills may later execute real workflows; permission/budget/stop/audit must be documented.'],
    },
    screenshot: '/docs/screenshots/fa/operator-skills.png',
  },
  {
    id: 'operator-learning',
    group: 'operator',
    icon: 'brain',
    title: { fa: 'Operator Learning / Methodology Records', en: 'Operator Learning / Methodology Records' },
    route: '/operator-learning',
    purpose: {
      fa: 'ثبت روش‌ها، تجربه‌ها و دستورالعمل‌های کاربر که انتخاب و اجرای skillها را برای target/project/user هدایت می‌کند.',
      en: 'Store user methodologies and experience that guide skill selection and execution for target/project/user scopes.',
    },
    features: {
      fa: ['Scope filter', 'Status filter', 'Bug class', 'Skill slug', 'Create methodology', 'Edit', 'Delete', 'confidence', 'use_count'],
      en: ['Scope filter', 'Status filter', 'Bug class', 'Skill slug', 'Create methodology', 'Edit', 'Delete', 'confidence', 'use_count'],
    },
    howToUse: {
      fa: ['Operator Learning را باز کنید.', 'methodology جدید بسازید.', 'bug_class یا skill_slug مرتبط را وارد کنید.', 'در Target Skill Profile آن را برای target فعال کنید.'],
      en: ['Open Operator Learning.', 'Create a new methodology.', 'Set matching bug_class or skill_slug.', 'Enable it in Target Skill Profile for a target.'],
    },
    outputs: {
      fa: ['learning record', 'methodology context', 'operator prompt influence', 'preferred learning records'],
      en: ['learning record', 'methodology context', 'operator prompt influence', 'preferred learning records'],
    },
    errors: {
      fa: ['scope اشتباه', 'record disabled', 'methodology خیلی کلی', 'عدم match با skill/bug_class'],
      en: ['wrong scope', 'disabled record', 'overly generic methodology', 'no skill/bug_class match'],
    },
    security: {
      fa: ['Learning record خودش vulnerability evidence نیست؛ فقط guidance برای reasoning است.'],
      en: ['A learning record is not vulnerability evidence by itself; it is reasoning guidance.'],
    },
    screenshot: '/docs/screenshots/fa/operator-learning.png',
  },
  {
    id: 'bug-class-validation',
    group: 'operator',
    icon: 'terminal',
    title: { fa: 'Bug-Class Validation Runtimes', en: 'Bug-Class Validation Runtimes' },
    route: 'AI Operator → skill_execution',
    purpose: {
      fa: 'اجرای controlled Level 2 evidence runtime برای کلاس‌های XSS، DOM XSS، CRLF، cache، open redirect، path traversal و CORS/CJ/CSRF.',
      en: 'Run controlled Level 2 evidence runtimes for XSS, DOM XSS, CRLF, cache, open redirect, path traversal, and CORS/CJ/CSRF.',
    },
    features: {
      fa: ['xss_reflection_context', 'dom_xss', 'crlf_header_injection', 'cache_poisoning_deception', 'open_redirect_chain', 'path_traversal_file_read_baseline', 'cors_clickjacking_csrf'],
      en: ['xss_reflection_context', 'dom_xss', 'crlf_header_injection', 'cache_poisoning_deception', 'open_redirect_chain', 'path_traversal_file_read_baseline', 'cors_clickjacking_csrf'],
    },
    howToUse: {
      fa: ['در Operator درخواست validation بدهید.', 'skill_execution را بازبینی کنید.', 'runtime_scope و execution_level را بررسی کنید.', 'observationها و next_step را بخوانید.'],
      en: ['Ask the Operator for validation.', 'Review skill_execution.', 'Check runtime_scope and execution_level.', 'Read observations and next_step.'],
    },
    outputs: {
      fa: ['controlled marker evidence', 'header evidence', 'source/sink evidence', 'baseline behavior', 'observation_ids', 'inconclusive/blocked counts'],
      en: ['controlled marker evidence', 'header evidence', 'source/sink evidence', 'baseline behavior', 'observation_ids', 'inconclusive/blocked counts'],
    },
    errors: {
      fa: ['no candidates', 'blocked/WAF', 'inconclusive', 'policy approval required', 'missing auth context'],
      en: ['no candidates', 'blocked/WAF', 'inconclusive', 'policy approval required', 'missing auth context'],
    },
    security: {
      fa: ['این runtimeها browser proof، raw CRLF، cache poisoning، sensitive file-read یا state-changing CSRF اجرا نمی‌کنند.'],
      en: ['These runtimes do not execute browser proof, raw CRLF, cache poisoning, sensitive file-read, or state-changing CSRF.'],
    },
    screenshot: '/docs/screenshots/fa/bug-class-validation.png',
  },
  {
    id: 'nuclei',
    group: 'nuclei',
    icon: 'file',
    title: { fa: 'Nuclei Templates و Profiles', en: 'Nuclei Templates and Profiles' },
    route: '/nuclei-templates',
    purpose: {
      fa: 'مدیریت templateها، placementها، validation، custom templates و AI-assisted draft workflow.',
      en: 'Manage templates, placements, validation, custom templates, and AI-assisted draft workflow.',
    },
    features: {
      fa: ['Root/Shared/Safe/Fast/Exposure/Balanced/Misconfig/CVEs/Full/Custom placements', 'Search templates', 'Create template', 'Save', 'Validate', 'Delete', 'AI draft status/strategy'],
      en: ['Root/Shared/Safe/Fast/Exposure/Balanced/Misconfig/CVEs/Full/Custom placements', 'Search templates', 'Create template', 'Save', 'Validate', 'Delete', 'AI draft status/strategy'],
    },
    howToUse: {
      fa: ['Nuclei Templates را باز کنید.', 'placement را انتخاب کنید.', 'template را باز یا ایجاد کنید.', 'قبل از ذخیره validate کنید.', 'برای draftهای AI human review انجام دهید.'],
      en: ['Open Nuclei Templates.', 'Choose placement.', 'Open or create a template.', 'Validate before saving.', 'Human-review AI drafts.'],
    },
    outputs: {
      fa: ['template YAML', 'validation result', 'placement', 'strategy signals', 'draft output'],
      en: ['template YAML', 'validation result', 'placement', 'strategy signals', 'draft output'],
    },
    errors: {
      fa: ['YAML invalid', 'nuclei validation failed', 'placement اشتباه', 'AI draft disabled'],
      en: ['invalid YAML', 'nuclei validation failed', 'wrong placement', 'AI draft disabled'],
    },
    security: {
      fa: ['Template جدید نباید destructive یا out-of-scope باشد؛ اجرای خودکار بدون approval ممنوع است.'],
      en: ['New templates must not be destructive or out-of-scope; auto-execution requires approval.'],
    },
    screenshot: '/docs/screenshots/fa/nuclei-templates.png',
  },
  {
    id: 'account',
    group: 'settings',
    icon: 'user',
    title: { fa: 'Account، Provider Keys و تنظیمات شخصی', en: 'Account, Provider Keys, and Personal Settings' },
    route: '/account',
    purpose: {
      fa: 'نمایش اطلاعات کاربر، تغییر رمز، scan queue شخصی، provider keys، feature flags و notification/provider configs.',
      en: 'View user data, change password, inspect personal scan queue, provider keys, feature flags, and notification/provider configs.',
    },
    features: {
      fa: ['Profile info', 'Change password', 'My Scan Queue', 'Subfinder providers', 'LLM providers', 'Telegram config', 'Feature flags'],
      en: ['Profile info', 'Change password', 'My Scan Queue', 'Subfinder providers', 'LLM providers', 'Telegram config', 'Feature flags'],
    },
    howToUse: {
      fa: ['Account را باز کنید.', 'برای تغییر رمز current و new password وارد کنید.', 'Provider keyها را با show/hide مدیریت کنید.', 'Feature flagهای account را فقط با آگاهی تغییر دهید.'],
      en: ['Open Account.', 'Enter current and new password to change password.', 'Manage provider keys with show/hide.', 'Change account feature flags carefully.'],
    },
    outputs: {
      fa: ['profile data', 'password change status', 'saved provider configs', 'effective feature flags'],
      en: ['profile data', 'password change status', 'saved provider configs', 'effective feature flags'],
    },
    errors: {
      fa: ['current password incorrect', 'provider key invalid', 'feature flag conflict', 'permission denied'],
      en: ['current password incorrect', 'invalid provider key', 'feature flag conflict', 'permission denied'],
    },
    security: {
      fa: ['API keyها، bot tokenها و رمزها نباید در screenshot یا docs دیده شوند.'],
      en: ['API keys, bot tokens, and passwords must not appear in screenshots or docs.'],
    },
    screenshot: '/docs/screenshots/fa/account-page.png',
  },
  {
    id: 'system-config',
    group: 'settings',
    icon: 'settings',
    title: { fa: 'System Config و تنظیمات Admin', en: 'System Config and Admin Settings' },
    route: '/settings',
    purpose: {
      fa: 'تنظیم global config، users، queue، concurrency، wordlists، resolvers، LLM، Telegram، VirusTotal، monitoring و logs.',
      en: 'Configure global config, users, queue, concurrency, wordlists, resolvers, LLM, Telegram, VirusTotal, monitoring, and logs.',
    },
    features: {
      fa: ['Users', 'Queue Manager', 'Concurrency Config', 'Wordlists', 'PureDNS Resolver Config', 'LLM Provider Config', 'Telegram', 'VirusTotal', 'Monitoring Server', 'System Logs', 'Feature Flags'],
      en: ['Users', 'Queue Manager', 'Concurrency Config', 'Wordlists', 'PureDNS Resolver Config', 'LLM Provider Config', 'Telegram', 'VirusTotal', 'Monitoring Server', 'System Logs', 'Feature Flags'],
    },
    howToUse: {
      fa: ['Settings را فقط با admin باز کنید.', 'هر panel را جدا تنظیم کنید.', 'بعد از تغییر wordlist/resolver تست کوچک بزنید.', 'logs و monitoring را برای خطا بررسی کنید.'],
      en: ['Open Settings as admin.', 'Configure each panel separately.', 'Run a small validation after wordlist/resolver changes.', 'Use logs and monitoring for troubleshooting.'],
    },
    outputs: {
      fa: ['global config', 'user list', 'queue state', 'wordlist imports', 'resolver pool', 'logs'],
      en: ['global config', 'user list', 'queue state', 'wordlist imports', 'resolver pool', 'logs'],
    },
    errors: {
      fa: ['upload limit', 'wordlist import failed', 'resolver slow', 'queue stuck', 'provider auth error'],
      en: ['upload limit', 'wordlist import failed', 'slow resolver', 'stuck queue', 'provider auth error'],
    },
    security: {
      fa: ['تغییرات system-wide باید audit شود و secrets در UI/screenshot نمایش داده نشوند.'],
      en: ['System-wide changes should be audited and secrets must not be exposed in UI/screenshots.'],
    },
    screenshot: '/docs/screenshots/fa/system-config.png',
  },
  {
    id: 'troubleshooting',
    group: 'support',
    icon: 'filter',
    title: { fa: 'Troubleshooting و خطاهای رایج', en: 'Troubleshooting and Common Issues' },
    route: 'Documentation → Troubleshooting',
    purpose: {
      fa: 'راهنمای تشخیص خطا برای route، login، API، recon، DNS، PureDNS، Operator runtime، frontend deploy و nginx/cache.',
      en: 'Troubleshoot route, login, API, recon, DNS, PureDNS, Operator runtime, frontend deploy, and nginx/cache issues.',
    },
    features: {
      fa: ['Route smoke', 'API status', 'backend logs', 'nginx restart', 'frontend bundle marker', 'Active Processes', 'PureDNS progress', 'Operator skill_execution'],
      en: ['Route smoke', 'API status', 'backend logs', 'nginx restart', 'frontend bundle marker', 'Active Processes', 'PureDNS progress', 'Operator skill_execution'],
    },
    howToUse: {
      fa: ['اول status code را چک کنید.', 'اگر SPA route است bundle marker را بررسی کنید.', 'برای backend خطا logs را ببینید.', 'برای Operator output_json و observations را بررسی کنید.'],
      en: ['Check status code first.', 'For SPA routes, inspect bundle markers.', 'For backend errors, read logs.', 'For Operator issues, inspect output_json and observations.'],
    },
    outputs: {
      fa: ['علت احتمالی خطا', 'مسیر debug', 'command یا smoke مناسب', 'next action'],
      en: ['Likely cause', 'debug path', 'relevant command or smoke', 'next action'],
    },
    errors: {
      fa: ['404 route', 'stale frontend asset', 'Cloudflare/nginx cache', 'backend restart needed', 'DB migration mismatch'],
      en: ['404 route', 'stale frontend asset', 'Cloudflare/nginx cache', 'backend restart needed', 'DB migration mismatch'],
    },
    security: {
      fa: ['در log و screenshot اطلاعات حساس را sanitize کنید.'],
      en: ['Sanitize sensitive data in logs and screenshots.'],
    },
    screenshot: '/docs/screenshots/fa/troubleshooting.png',
  },
  {
    id: 'create-target-modal',
    group: 'targets',
    icon: 'globe',
    title: { fa: 'Create Target Modal', en: 'Create Target Modal' },
    route: '/targets → New Target',
    purpose: {
      fa: 'ساخت Target جدید و انتخاب ماژول‌های discovery، crawl، PureDNS، Nuclei و sourceهای اطلاعاتی.',
      en: 'Create a new target and select discovery, crawl, PureDNS, Nuclei, and intelligence source modules.',
    },
    features: {
      fa: ['Name', 'Root Domain', 'Description', 'Frequency', 'AlterX', 'Waymore', 'GAU', 'Katana', 'VirusTotal', 'Port Scan', 'Cero', 'crt.sh', 'PureDNS', 'AbuseDB', 'Amass', 'Nuclei', 'Nuclei Profile', 'PureDNS Wordlists'],
      en: ['Name', 'Root Domain', 'Description', 'Frequency', 'AlterX', 'Waymore', 'GAU', 'Katana', 'VirusTotal', 'Port Scan', 'Cero', 'crt.sh', 'PureDNS', 'AbuseDB', 'Amass', 'Nuclei', 'Nuclei Profile', 'PureDNS Wordlists'],
    },
    howToUse: {
      fa: ['از صفحه Targets روی New Target بزنید.', 'root domain مجاز را وارد کنید.', 'ماژول‌ها را مطابق scope و مجوز انتخاب کنید.', 'اگر PureDNS فعال است wordlist مناسب را انتخاب کنید.', 'اگر Nuclei فعال است profile مناسب را انتخاب کنید.', 'Target را ذخیره کنید و سپس scan را اجرا کنید.'],
      en: ['Open Targets and click New Target.', 'Enter the authorized root domain.', 'Select modules according to scope and authorization.', 'Select wordlists when PureDNS is enabled.', 'Select a Nuclei profile when Nuclei is enabled.', 'Save the target and run scan when ready.'],
    },
    outputs: {
      fa: ['Target جدید', 'تنظیمات moduleها', 'آماده شدن target برای scan runtime', 'انتخاب wordlist/profile برای discovery و Nuclei'],
      en: ['New target', 'module settings', 'target ready for scan runtime', 'selected wordlist/profile for discovery and Nuclei'],
    },
    errors: {
      fa: ['root domain نامعتبر', 'target تکراری', 'PureDNS بدون resolver/wordlist مناسب', 'VirusTotal بدون API key', 'Nuclei بدون template/profile معتبر'],
      en: ['invalid root domain', 'duplicate target', 'PureDNS without proper resolver/wordlist', 'VirusTotal without API key', 'Nuclei without valid template/profile'],
    },
    security: {
      fa: ['فقط target مجاز بسازید. port scan، active crawl، PureDNS بزرگ و Nuclei باید با scope و مجوز هماهنگ باشند.'],
      en: ['Create authorized targets only. Port scan, active crawl, large PureDNS, and Nuclei must match scope and authorization.'],
    },
    screenshot: '/docs/screenshots/fa/targets-create-target-modal.png',
  },
  {
    id: 'edit-target-modal',
    group: 'targets',
    icon: 'globe',
    title: { fa: 'Edit Target Modal', en: 'Edit Target Modal' },
    route: '/targets → Edit Target',
    purpose: {
      fa: 'ویرایش metadata و تنظیمات moduleهای target برای scanهای بعدی.',
      en: 'Edit target metadata and module settings for future scans.',
    },
    features: {
      fa: ['ویرایش name/root_domain/description', 'تغییر moduleها', 'تغییر nuclei_profile', 'تغییر puredns_wordlists', 'تغییر in_scope در صورت وجود'],
      en: ['edit name/root_domain/description', 'change modules', 'change nuclei_profile', 'change puredns_wordlists', 'change in_scope when available'],
    },
    howToUse: {
      fa: ['در Targets روی edit بزنید.', 'فیلدهای لازم را تغییر دهید.', 'تفاوت تغییر config با حذف evidence قبلی را در نظر بگیرید.', 'ذخیره کنید.'],
      en: ['Click edit in Targets.', 'Change required fields.', 'Understand that config changes do not necessarily delete previous evidence.', 'Save changes.'],
    },
    outputs: {
      fa: ['Target config جدید', 'اثر روی scanهای آینده', 'حفظ evidence قبلی مگر implementation خلافش را انجام دهد'],
      en: ['updated target config', 'effect on future scans', 'previous evidence remains unless implementation explicitly removes it'],
    },
    errors: {
      fa: ['root domain نامعتبر', 'تغییر حین scan فعال', 'انتظار اشتباه از پاک شدن داده‌های قبلی'],
      en: ['invalid root domain', 'change during active scan', 'wrong expectation that old data is removed'],
    },
    security: {
      fa: ['تغییر scope باید عمدی و مستند باشد. target را بدون مجوز in-scope نکنید.'],
      en: ['Scope changes must be intentional and documented. Do not mark unauthorized targets in-scope.'],
    },
    screenshot: '/docs/screenshots/fa/targets-edit-target-modal.png',
  },
  {
    id: 'target-import-export',
    group: 'targets',
    icon: 'database',
    title: { fa: 'Import / Export Targets', en: 'Import / Export Targets' },
    route: '/targets → Import / Export',
    purpose: {
      fa: 'انتقال، backup یا review آفلاین targetها و داده‌های مرتبط.',
      en: 'Move, back up, or review targets and related data offline.',
    },
    features: {
      fa: ['Import JSON', 'Export JSON', 'version', 'export_date', 'targets', 'assets', 'urls', 'module settings'],
      en: ['Import JSON', 'Export JSON', 'version', 'export_date', 'targets', 'assets', 'urls', 'module settings'],
    },
    howToUse: {
      fa: ['برای backup روی Export بزنید.', 'فایل خروجی را امن نگه دارید.', 'برای restore یا migration از Import استفاده کنید.', 'بعد از import target و evidence را review کنید.'],
      en: ['Click Export for backup.', 'Store the file securely.', 'Use Import for restore or migration.', 'Review target and evidence after import.'],
    },
    outputs: {
      fa: ['فایل export', 'target واردشده', 'asset/urlهای واردشده در صورت پشتیبانی schema'],
      en: ['export file', 'imported target', 'imported assets/URLs when schema supports them'],
    },
    errors: {
      fa: ['JSON نامعتبر', 'version ناسازگار', 'duplicate target', 'partial import'],
      en: ['invalid JSON', 'unsupported version', 'duplicate target', 'partial import'],
    },
    security: {
      fa: ['Export شامل داده حساس attack surface است. آن را public commit یا share نکنید.'],
      en: ['Exports contain sensitive attack-surface data. Do not commit or share them publicly.'],
    },
    screenshot: '/docs/screenshots/fa/targets-import-modal.png',
  },
  {
    id: 'recon-discovery-pipeline',
    group: 'target-detail',
    icon: 'terminal',
    title: { fa: 'Recon / Discovery Pipeline', en: 'Recon / Discovery Pipeline' },
    route: '/targets/:id → scan/discovery modules',
    purpose: {
      fa: 'جریان discovery از sourceهای passive، DNS validation، PureDNS brute-force، wildcard filtering، AlterX mutation و URL collection.',
      en: 'Discovery flow across passive sources, DNS validation, PureDNS brute force, wildcard filtering, AlterX mutation, and URL collection.',
    },
    features: {
      fa: ['Subfinder', 'Assetfinder', 'crt.sh', 'Cero', 'AbuseDB', 'Amass', 'DNSX', 'PureDNS', 'Wildcard Filter', 'AlterX', 'Wayback', 'GAU', 'Katana', 'Waymore', 'VirusTotal'],
      en: ['Subfinder', 'Assetfinder', 'crt.sh', 'Cero', 'AbuseDB', 'Amass', 'DNSX', 'PureDNS', 'Wildcard Filter', 'AlterX', 'Wayback', 'GAU', 'Katana', 'Waymore', 'VirusTotal'],
    },
    howToUse: {
      fa: ['Target را با moduleهای مناسب بسازید.', 'scan را اجرا کنید.', 'Passive candidates توسط DNSX validate می‌شوند.', 'PureDNS brute-force خروجی خودش را resolve/live-validate می‌کند.', 'AlterX بعد از live validation روی hosts معتبر اجرا می‌شود.', 'نتیجه را در Assets و URLs بررسی کنید.'],
      en: ['Create target with appropriate modules.', 'Run scan.', 'Passive candidates are validated by DNSX.', 'PureDNS brute-force resolves/live-validates its own output.', 'AlterX runs after live validation on trusted hosts.', 'Review results in Assets and URLs.'],
    },
    outputs: {
      fa: ['subdomain candidates', 'live assets', 'URLs', 'source attribution', 'progress/runtime evidence'],
      en: ['subdomain candidates', 'live assets', 'URLs', 'source attribution', 'progress/runtime evidence'],
    },
    errors: {
      fa: ['DNS wildcard noise', 'resolver کند', 'PureDNS ETA طولانی', 'URL source بدون نتیجه', 'false live اگر parser اشتباه باشد'],
      en: ['DNS wildcard noise', 'slow resolvers', 'long PureDNS ETA', 'empty URL sources', 'false live if parser is wrong'],
    },
    security: {
      fa: ['Discovery باید در scope باشد. brute-force بزرگ باید با resolver، rate و مجوز هماهنگ باشد.'],
      en: ['Discovery must remain in scope. Large brute force requires resolver capacity, rate controls, and authorization.'],
    },
    screenshot: '/docs/screenshots/fa/recon-start-controls.png',
  },
  {
    id: 'active-processes-puredns',
    group: 'settings',
    icon: 'terminal',
    title: { fa: 'Active Processes و PureDNS Progress', en: 'Active Processes and PureDNS Progress' },
    route: 'Dashboard / Target / Settings → Monitoring / Progress',
    purpose: {
      fa: 'نمایش وضعیت jobهای فعال، command، duration، progress، rate و ETA برای debug و monitoring.',
      en: 'Display active jobs, command, duration, progress, rate, and ETA for debugging and monitoring.',
    },
    features: {
      fa: ['Active command', 'PID', 'duration', 'target context', 'PureDNS rate', 'PureDNS ETA', 'progress telemetry', 'monitoring stats'],
      en: ['active command', 'PID', 'duration', 'target context', 'PureDNS rate', 'PureDNS ETA', 'progress telemetry', 'monitoring stats'],
    },
    howToUse: {
      fa: ['هنگام scan فعال processها را بررسی کنید.', 'برای PureDNS نرخ و ETA را بخوانید.', 'اگر کند است resolver pool و wordlist size را بررسی کنید.', 'برای stuck job logs و queue را بررسی کنید.'],
      en: ['Review processes during active scan.', 'Read PureDNS rate and ETA.', 'If slow, inspect resolver pool and wordlist size.', 'For stuck jobs, inspect logs and queue.'],
    },
    outputs: {
      fa: ['وضعیت runtime', 'progress قابل فهم', 'تشخیص bottleneck در resolver/wordlist/server'],
      en: ['runtime status', 'readable progress', 'bottleneck diagnosis for resolver/wordlist/server'],
    },
    errors: {
      fa: ['process stale', 'PID قدیمی', 'resolver throttling', 'wordlist خیلی بزرگ', 'logs ناکافی'],
      en: ['stale process', 'old PID', 'resolver throttling', 'oversized wordlist', 'insufficient logs'],
    },
    security: {
      fa: ['command/log ممکن است path یا target حساس داشته باشد؛ screenshot را sanitize کنید.'],
      en: ['commands/logs may include sensitive paths or targets; sanitize screenshots.'],
    },
    screenshot: '/docs/screenshots/fa/recon-puredns-progress.png',
  },

  {
    id: 'ai-analysis-panel',
    group: 'operator',
    icon: 'brain',
    title: { fa: 'AI Analysis Panel', en: 'AI Analysis Panel' },
    route: '/targets/:id → Analysis → AI Analysis',
    purpose: {
      fa: 'خلاصه‌سازی و تفسیر AI از شواهد target، تکنولوژی‌ها، assetها، URLها و سیگنال‌های سطح حمله.',
      en: 'AI-assisted summary and interpretation of target evidence, technologies, assets, URLs, and attack-surface signals.',
    },
    features: {
      fa: ['target summary', 'technology interpretation', 'risk hints', 'attack-surface observations', 'evidence-aware narrative'],
      en: ['target summary', 'technology interpretation', 'risk hints', 'attack-surface observations', 'evidence-aware narrative'],
    },
    howToUse: {
      fa: ['Analysis tab را باز کنید.', 'AI Analysis را بررسی کنید.', 'خلاصه را با Assets، URLs و Findings تطبیق دهید.', 'برای تست عملی از Operator Chat استفاده کنید.'],
      en: ['Open Analysis tab.', 'Review AI Analysis.', 'Cross-check the summary with Assets, URLs, and Findings.', 'Use Operator Chat for actionable testing.'],
    },
    outputs: {
      fa: ['summary', 'risk hints', 'technology notes', 'next-review suggestions'],
      en: ['summary', 'risk hints', 'technology notes', 'next-review suggestions'],
    },
    errors: {
      fa: ['LLM provider تنظیم نشده', 'evidence کافی نیست', 'feature flag غیرفعال است', 'خروجی generic'],
      en: ['LLM provider not configured', 'insufficient evidence', 'feature flag disabled', 'generic output'],
    },
    security: {
      fa: ['AI Analysis خودش finding قطعی نیست؛ فقط تفسیر و guidance است.'],
      en: ['AI Analysis is not a confirmed finding by itself; it is interpretation and guidance.'],
    },
    screenshot: '/docs/screenshots/fa/ai-analysis-panel.png',
  },
  {
    id: 'recommendations-panel',
    group: 'operator',
    icon: 'brain',
    title: { fa: 'Recommendations Panel', en: 'Recommendations Panel' },
    route: '/targets/:id → Analysis → Recommendations',
    purpose: {
      fa: 'پیشنهاد next step برای recon، evidence review، controlled validation و اولویت‌بندی سطح حمله.',
      en: 'Recommend next steps for recon, evidence review, controlled validation, and attack-surface prioritization.',
    },
    features: {
      fa: ['next-step suggestions', 'priority hints', 'evidence gaps', 'manual review guidance'],
      en: ['next-step suggestions', 'priority hints', 'evidence gaps', 'manual review guidance'],
    },
    howToUse: {
      fa: ['Recommendations را باز کنید.', 'پیشنهادها را با evidence واقعی مقایسه کنید.', 'پیشنهادهای مناسب را در Operator Chat یا workflow مربوطه دنبال کنید.'],
      en: ['Open Recommendations.', 'Compare suggestions with real evidence.', 'Follow suitable suggestions in Operator Chat or the relevant workflow.'],
    },
    outputs: {
      fa: ['recommended actions', 'rationale', 'missing evidence hints'],
      en: ['recommended actions', 'rationale', 'missing evidence hints'],
    },
    errors: {
      fa: ['پیشنهاد generic', 'evidence کم', 'feature flag غیرفعال', 'عدم ارتباط با target فعلی'],
      en: ['generic recommendation', 'low evidence', 'feature flag disabled', 'not relevant to current target'],
    },
    security: {
      fa: ['Recommendation به معنی مجوز اجرا نیست؛ قبل از action، scope و policy را بررسی کنید.'],
      en: ['A recommendation is not execution authorization; review scope and policy before action.'],
    },
    screenshot: '/docs/screenshots/fa/recommendations-panel.png',
  },
  {
    id: 'agent-actions-panel',
    group: 'operator',
    icon: 'terminal',
    title: { fa: 'Agent Actions Panel', en: 'Agent Actions Panel' },
    route: '/targets/:id → Analysis → Agent Actions',
    purpose: {
      fa: 'نمایش actionهای پیشنهادی یا اجراشده توسط agent/operator، وضعیت approval، runtime labels و نتیجه dispatch.',
      en: 'Display proposed or executed agent/operator actions, approval status, runtime labels, and dispatch results.',
    },
    features: {
      fa: ['proposed action', 'approval required', 'executed by autopilot', 'runtime labels', 'unsupported preview-only action', 'controlled run/result IDs'],
      en: ['proposed action', 'approval required', 'executed by autopilot', 'runtime labels', 'unsupported preview-only action', 'controlled run/result IDs'],
    },
    howToUse: {
      fa: ['Agent Actions را باز کنید.', 'status هر action را بخوانید.', 'اگر approval required است فقط در صورت مجوز تایید کنید.', 'runtime evidence را با Findings/Operator خروجی تطبیق دهید.'],
      en: ['Open Agent Actions.', 'Read each action status.', 'Approve only when authorized.', 'Cross-check runtime evidence with Findings/Operator output.'],
    },
    outputs: {
      fa: ['action status', 'controlled run ID', 'controlled result ID', 'policy decision', 'runtime evidence summary'],
      en: ['action status', 'controlled run ID', 'controlled result ID', 'policy decision', 'runtime evidence summary'],
    },
    errors: {
      fa: ['unsupported action', 'policy blocked', 'approval required', 'runtime failed', 'inconclusive evidence'],
      en: ['unsupported action', 'policy blocked', 'approval required', 'runtime failed', 'inconclusive evidence'],
    },
    security: {
      fa: ['Unsupported actionها preview هستند. Actionهای واقعی باید policy/scope/approval/rate/budget را رعایت کنند.'],
      en: ['Unsupported actions are previews. Real actions must respect policy, scope, approval, rate, and budget controls.'],
    },
    screenshot: '/docs/screenshots/fa/agent-actions-panel.png',
  },
  {
    id: 'bug-tests-panel',
    group: 'operator',
    icon: 'terminal',
    title: { fa: 'Bug Tests Panel', en: 'Bug Tests Panel' },
    route: '/targets/:id → Analysis → Bug Tests',
    purpose: {
      fa: 'بررسی run/result تست‌های bug، evidence، pattern/payload reference و وضعیت نتیجه.',
      en: 'Review bug test runs/results, evidence, pattern/payload references, and result state.',
    },
    features: {
      fa: ['test runs', 'test results', 'bug type', 'evidence', 'pattern reference', 'payload reference', 'blocked/inconclusive handling'],
      en: ['test runs', 'test results', 'bug type', 'evidence', 'pattern reference', 'payload reference', 'blocked/inconclusive handling'],
    },
    howToUse: {
      fa: ['Bug Tests را باز کنید.', 'run و result را بررسی کنید.', 'evidence را بخوانید.', 'blocked یا inconclusive را vulnerability حساب نکنید.'],
      en: ['Open Bug Tests.', 'Review run and result.', 'Read evidence.', 'Do not treat blocked or inconclusive as confirmed vulnerabilities.'],
    },
    outputs: {
      fa: ['bug test run', 'bug test result', 'evidence JSON', 'status', 'classification'],
      en: ['bug test run', 'bug test result', 'evidence JSON', 'status', 'classification'],
    },
    errors: {
      fa: ['false positive', 'inconclusive result', 'blocked response', 'missing candidate', 'unsupported test type'],
      en: ['false positive', 'inconclusive result', 'blocked response', 'missing candidate', 'unsupported test type'],
    },
    security: {
      fa: ['تست‌های فعال باید authorization و policy داشته باشند و evidence باید قابل تکرار باشد.'],
      en: ['Active tests require authorization and policy support, and evidence must be reproducible.'],
    },
    screenshot: '/docs/screenshots/fa/bug-tests-panel.png',
  },
  {
    id: 'pattern-payload-registries',
    group: 'operator',
    icon: 'database',
    title: { fa: 'Pattern و Payload Registry', en: 'Pattern and Payload Registry' },
    route: '/targets/:id → Analysis → Pattern Registry / Payload Registry',
    purpose: {
      fa: 'مدیریت و بررسی patternها و payload metadata برای bug-class reasoning، strategy و validation planning.',
      en: 'Review pattern and payload metadata for bug-class reasoning, strategy, and validation planning.',
    },
    features: {
      fa: ['pattern packs', 'pattern keys', 'bug class', 'tags', 'payload packs', 'payload key', 'safety class', 'intended context', 'metadata-only payloads'],
      en: ['pattern packs', 'pattern keys', 'bug class', 'tags', 'payload packs', 'payload key', 'safety class', 'intended context', 'metadata-only payloads'],
    },
    howToUse: {
      fa: ['Registry را باز کنید.', 'pattern یا payload مرتبط با bug class را بررسی کنید.', 'metadata را برای strategy بخوانید.', 'اجرای واقعی payload را فقط با policy و authorization انجام دهید.'],
      en: ['Open the registry.', 'Review patterns or payloads related to the bug class.', 'Use metadata for strategy.', 'Execute payloads only with policy and authorization.'],
    },
    outputs: {
      fa: ['pattern metadata', 'payload metadata', 'tags', 'safety class', 'strategy signals'],
      en: ['pattern metadata', 'payload metadata', 'tags', 'safety class', 'strategy signals'],
    },
    errors: {
      fa: ['payload metadata بدون runtime', 'pattern خیلی کلی', 'bug class mismatch', 'اجرای payload بدون authorization'],
      en: ['payload metadata without runtime', 'overly generic pattern', 'bug class mismatch', 'payload execution without authorization'],
    },
    security: {
      fa: ['Registry خودش اجرای payload نیست. اجرای payload باید policy-gated، audited و scope-aware باشد.'],
      en: ['The registry is not payload execution. Payload execution must be policy-gated, audited, and scope-aware.'],
    },
    screenshot: '/docs/screenshots/fa/pattern-payload-registries.png',
  },
  {
    id: 'findings-evidence-json',
    group: 'target-detail',
    icon: 'shield',
    title: { fa: 'Finding Evidence JSON', en: 'Finding Evidence JSON' },
    route: '/targets/:id → Findings → evidence_json',
    purpose: {
      fa: 'خواندن evidence_json برای فهم دقیق claim، request/response، confidence، source و reproduction context.',
      en: 'Read evidence_json to understand the claim, request/response, confidence, source, and reproduction context.',
    },
    features: {
      fa: ['evidence_json', 'source_tool', 'severity', 'confidence', 'status', 'triage_note', 'reproduction data'],
      en: ['evidence_json', 'source_tool', 'severity', 'confidence', 'status', 'triage_note', 'reproduction data'],
    },
    howToUse: {
      fa: ['Finding را باز کنید.', 'evidence_json را بخوانید.', 'status/severity را فقط با evidence کافی تغییر دهید.', 'برای report، sensitive data را sanitize کنید.'],
      en: ['Open the finding.', 'Read evidence_json.', 'Change status/severity only with enough evidence.', 'Sanitize sensitive data for reports.'],
    },
    outputs: {
      fa: ['evidence fields', 'triage decision', 'exportable finding data'],
      en: ['evidence fields', 'triage decision', 'exportable finding data'],
    },
    errors: {
      fa: ['evidence ناقص', 'claim بدون reproduction', 'blocked response', 'false positive', 'severity overclaim'],
      en: ['incomplete evidence', 'claim without reproduction', 'blocked response', 'false positive', 'severity overclaim'],
    },
    security: {
      fa: ['Evidence ممکن است sensitive باشد. بدون مجوز share نکنید و برای screenshot/report آن را sanitize کنید.'],
      en: ['Evidence may be sensitive. Do not share without authorization and sanitize screenshots/reports.'],
    },
    screenshot: '/docs/screenshots/fa/findings-evidence-json.png',
  },

  {
    id: 'settings-users-panel',
    group: 'settings',
    icon: 'user',
    title: { fa: 'Users Panel', en: 'Users Panel' },
    route: '/settings → Users',
    purpose: {
      fa: 'مدیریت کاربران توسط admin: ساخت، ویرایش، حذف، role و محدودیت concurrent scan.',
      en: 'Admin user management: create, edit, delete, role assignment, and concurrent scan limits.',
    },
    features: {
      fa: ['user list', 'create user', 'edit user', 'delete user', 'role', 'password', 'max concurrent scans', 'scrollable user modal'],
      en: ['user list', 'create user', 'edit user', 'delete user', 'role', 'password', 'max concurrent scans', 'scrollable user modal'],
    },
    howToUse: {
      fa: ['با admin وارد شوید.', 'System Config را باز کنید.', 'Users panel را انتخاب کنید.', 'کاربر جدید بسازید یا کاربر موجود را ویرایش کنید.', 'role و scan slots را با دقت تنظیم کنید.'],
      en: ['Log in as admin.', 'Open System Config.', 'Select Users panel.', 'Create or edit a user.', 'Set role and scan slots carefully.'],
    },
    outputs: {
      fa: ['user record', 'role update', 'scan slot limit', 'deleted user state'],
      en: ['user record', 'role update', 'scan slot limit', 'deleted user state'],
    },
    errors: {
      fa: ['duplicate username', 'password خالی یا ضعیف', 'permission denied', 'role نامعتبر', 'modal overflow'],
      en: ['duplicate username', 'empty or weak password', 'permission denied', 'invalid role', 'modal overflow'],
    },
    security: {
      fa: ['رمزها و اطلاعات واقعی کاربران را در screenshot نشان ندهید. فقط admin باید به این panel دسترسی داشته باشد.'],
      en: ['Do not expose passwords or real user data in screenshots. Only admins should access this panel.'],
    },
    screenshot: '/docs/screenshots/fa/settings-users.png',
  },
  {
    id: 'settings-queue-manager',
    group: 'settings',
    icon: 'terminal',
    title: { fa: 'Queue Manager', en: 'Queue Manager' },
    route: '/settings → Queue Manager',
    purpose: {
      fa: 'مدیریت jobهای در صف: مشاهده، حذف، clear و تغییر اولویت queue.',
      en: 'Manage queued jobs: inspect, remove, clear, and reorder queue items.',
    },
    features: {
      fa: ['queue items', 'position/index', 'payload', 'module', 'target id', 'target name', 'owner', 'remove item', 'clear queue', 'move top/bottom'],
      en: ['queue items', 'position/index', 'payload', 'module', 'target id', 'target name', 'owner', 'remove item', 'clear queue', 'move top/bottom'],
    },
    howToUse: {
      fa: ['Queue Manager را باز کنید.', 'jobهای pending را بررسی کنید.', 'برای job اشتباه remove بزنید.', 'برای پاکسازی کامل clear queue را فقط با اطمینان بزنید.', 'برای اولویت‌دهی move top/bottom استفاده کنید.'],
      en: ['Open Queue Manager.', 'Review pending jobs.', 'Remove unwanted jobs.', 'Use clear queue only when certain.', 'Use move top/bottom to prioritize.'],
    },
    outputs: {
      fa: ['queue state جدید', 'حذف یا جابه‌جایی job', 'اولویت اجرای جدید'],
      en: ['updated queue state', 'removed or reordered job', 'new execution priority'],
    },
    errors: {
      fa: ['item قبلاً اجرا شده', 'index اشتباه', 'clear queue تصادفی', 'توقع توقف process فعال در حالی که queue فقط pending را تغییر می‌دهد'],
      en: ['item already processed', 'wrong index', 'accidental clear queue', 'expecting active process stop while queue only changes pending items'],
    },
    security: {
      fa: ['Queue ممکن است jobهای کاربران دیگر را داشته باشد؛ قبل از تغییر owner و target را بررسی کنید.'],
      en: ['Queue may contain other users jobs; verify owner and target before changes.'],
    },
    screenshot: '/docs/screenshots/fa/settings-queue-manager.png',
  },
  {
    id: 'settings-concurrency',
    group: 'settings',
    icon: 'settings',
    title: { fa: 'Concurrency Config', en: 'Concurrency Config' },
    route: '/settings → Concurrency Config',
    purpose: {
      fa: 'تنظیم ظرفیت اجرای همزمان scanها و کنترل فشار روی سرور و targetها.',
      en: 'Configure scan concurrency to control server and target load.',
    },
    features: {
      fa: ['global concurrency', 'user scan slots', 'worker limits', 'resource capacity', 'scan scheduling impact'],
      en: ['global concurrency', 'user scan slots', 'worker limits', 'resource capacity', 'scan scheduling impact'],
    },
    howToUse: {
      fa: ['Concurrency panel را باز کنید.', 'ظرفیت فعلی را بررسی کنید.', 'عددها را با توان سرور و مجوز target تنظیم کنید.', 'بعد از تغییر monitoring و queue را بررسی کنید.'],
      en: ['Open Concurrency panel.', 'Review current capacity.', 'Tune values based on server capacity and target authorization.', 'Monitor queue and system after changes.'],
    },
    outputs: {
      fa: ['concurrency config جدید', 'رفتار متفاوت queue و workerها', 'اثر روی سرعت و فشار scan'],
      en: ['updated concurrency config', 'changed queue/worker behavior', 'impact on scan speed and load'],
    },
    errors: {
      fa: ['concurrency خیلی بالا و overload', 'concurrency خیلی پایین و کندی scan', 'تداخل با user scan slots'],
      en: ['too high concurrency causing overload', 'too low concurrency causing slow scans', 'conflict with user scan slots'],
    },
    security: {
      fa: ['افزایش concurrency حجم ترافیک را زیاد می‌کند؛ باید با rate limit و authorization هماهنگ باشد.'],
      en: ['Increasing concurrency raises traffic volume and must match rate limits and authorization.'],
    },
    screenshot: '/docs/screenshots/fa/settings-concurrency.png',
  },
  {
    id: 'settings-wordlists',
    group: 'settings',
    icon: 'database',
    title: { fa: 'Wordlists Config', en: 'Wordlists Config' },
    route: '/settings → Wordlists Config',
    purpose: {
      fa: 'مدیریت wordlistهای discovery و brute-force، شامل upload و import از URL به‌صورت async.',
      en: 'Manage discovery/brute-force wordlists, including upload and async URL imports.',
    },
    features: {
      fa: ['upload wordlist', 'URL import', 'async import job', 'file-backed storage', 'line count', 'file size', 'source type', 'import status', 'PureDNS wordlist selection'],
      en: ['upload wordlist', 'URL import', 'async import job', 'file-backed storage', 'line count', 'file size', 'source type', 'import status', 'PureDNS wordlist selection'],
    },
    howToUse: {
      fa: ['Wordlists panel را باز کنید.', 'فایل را upload کنید یا URL بدهید.', 'برای URL import منتظر job progress بمانید.', 'بعد از import، wordlist را در Create/Edit Target برای PureDNS انتخاب کنید.'],
      en: ['Open Wordlists panel.', 'Upload a file or provide a URL.', 'Wait for URL import job progress.', 'Select the wordlist in Create/Edit Target for PureDNS.'],
    },
    outputs: {
      fa: ['wordlist ذخیره‌شده', 'metadata', 'line count', 'import job status', 'wordlist قابل انتخاب برای target'],
      en: ['stored wordlist', 'metadata', 'line count', 'import job status', 'selectable target wordlist'],
    },
    errors: {
      fa: ['upload limit', 'URL download failed', 'import job failed', 'wordlist خالی', 'metadata mismatch', 'wordlist خیلی بزرگ برای resolver pool'],
      en: ['upload limit', 'URL download failed', 'import job failed', 'empty wordlist', 'metadata mismatch', 'wordlist too large for resolver pool'],
    },
    security: {
      fa: ['wordlist بزرگ حجم scan را زیاد می‌کند؛ با scope، rate و resolver capacity هماهنگ کنید.'],
      en: ['Large wordlists increase scan volume; align with scope, rate, and resolver capacity.'],
    },
    screenshot: '/docs/screenshots/fa/settings-wordlists.png',
  },
  {
    id: 'settings-puredns-resolvers',
    group: 'settings',
    icon: 'terminal',
    title: { fa: 'PureDNS Resolver Config', en: 'PureDNS Resolver Config' },
    route: '/settings → PureDNS Resolver Config',
    purpose: {
      fa: 'مدیریت resolverهای account-scoped برای PureDNS و بهبود سرعت/کیفیت brute-force discovery.',
      en: 'Manage account-scoped PureDNS resolvers to improve brute-force discovery speed and quality.',
    },
    features: {
      fa: ['resolver list', 'resolver count', 'account-scoped config', 'PureDNS throughput', 'progress telemetry', 'ETA', 'wildcard impact'],
      en: ['resolver list', 'resolver count', 'account-scoped config', 'PureDNS throughput', 'progress telemetry', 'ETA', 'wildcard impact'],
    },
    howToUse: {
      fa: ['Resolver Config را باز کنید.', 'resolverهای سالم و کافی اضافه کنید.', 'ذخیره کنید.', 'PureDNS scan را با wordlist مناسب اجرا کنید.', 'rate و ETA را بررسی کنید.'],
      en: ['Open Resolver Config.', 'Add enough healthy resolvers.', 'Save config.', 'Run PureDNS with an appropriate wordlist.', 'Review rate and ETA.'],
    },
    outputs: {
      fa: ['resolver config ذخیره‌شده', 'PureDNS progress', 'throughput بهتر', 'نتایج brute-force validated'],
      en: ['saved resolver config', 'PureDNS progress', 'better throughput', 'validated brute-force results'],
    },
    errors: {
      fa: ['resolver pool کوچک', 'resolver throttling', 'resolver خراب', 'PureDNS کند', 'wordlist بسیار بزرگ', 'wildcard DNS noise'],
      en: ['small resolver pool', 'resolver throttling', 'bad resolver', 'slow PureDNS', 'oversized wordlist', 'wildcard DNS noise'],
    },
    security: {
      fa: ['PureDNS brute-force باید مجاز، rate-aware و scope-bound باشد. خروجی PureDNS را پیش‌فرض دوباره DNSX نکنید مگر debug/optional.'],
      en: ['PureDNS brute-force must be authorized, rate-aware, and scope-bound. Do not DNSX revalidate PureDNS output by default unless debug/optional.'],
    },
    screenshot: '/docs/screenshots/fa/settings-puredns-resolvers.png',
  },
  {
    id: 'settings-llm-provider',
    group: 'settings',
    icon: 'brain',
    title: { fa: 'LLM Provider Config', en: 'LLM Provider Config' },
    route: '/settings یا /account → LLM Provider Config',
    purpose: {
      fa: 'تنظیم provider/model/API برای قابلیت‌های AI Analysis و AI Operator.',
      en: 'Configure provider/model/API settings for AI Analysis and AI Operator features.',
    },
    features: {
      fa: ['provider', 'display name', 'api_key_saved', 'base_url', 'default_model', 'enabled', 'is_default', 'scope/owner'],
      en: ['provider', 'display name', 'api_key_saved', 'base_url', 'default_model', 'enabled', 'is_default', 'scope/owner'],
    },
    howToUse: {
      fa: ['LLM panel را باز کنید.', 'provider و base URL را تنظیم کنید.', 'API key را وارد کنید.', 'model پیش‌فرض را مشخص کنید.', 'enabled/default را تنظیم کنید.'],
      en: ['Open LLM panel.', 'Set provider and base URL.', 'Enter API key.', 'Set default model.', 'Configure enabled/default state.'],
    },
    outputs: {
      fa: ['LLM provider ذخیره‌شده', 'فعال شدن AI features', 'اثر روی Operator و Analysis'],
      en: ['saved LLM provider', 'enabled AI features', 'impact on Operator and Analysis'],
    },
    errors: {
      fa: ['API key نامعتبر', 'base URL اشتباه', 'model unsupported', 'provider disabled', 'feature flag غیرفعال'],
      en: ['invalid API key', 'wrong base URL', 'unsupported model', 'provider disabled', 'feature flag disabled'],
    },
    security: {
      fa: ['API key را هرگز در screenshot/docs نشان ندهید. توجه کنید چه داده‌ای به LLM provider ارسال می‌شود.'],
      en: ['Never expose API keys in screenshots/docs. Be aware of what data is sent to the LLM provider.'],
    },
    screenshot: '/docs/screenshots/fa/settings-llm-provider-sanitized.png',
  },
  {
    id: 'settings-notifications-integrations',
    group: 'settings',
    icon: 'settings',
    title: { fa: 'Telegram و VirusTotal Config', en: 'Telegram and VirusTotal Config' },
    route: '/settings یا /account → Telegram / VirusTotal',
    purpose: {
      fa: 'تنظیم notificationها و enrichmentهای خارجی مثل Telegram و VirusTotal.',
      en: 'Configure notifications and external enrichment integrations such as Telegram and VirusTotal.',
    },
    features: {
      fa: ['Telegram enabled', 'bot token saved state', 'chat_id', 'notification events', 'fresh asset screenshot', 'VirusTotal API key', 'source attribution', 'rate limit'],
      en: ['Telegram enabled', 'bot token saved state', 'chat_id', 'notification events', 'fresh asset screenshot', 'VirusTotal API key', 'source attribution', 'rate limit'],
    },
    howToUse: {
      fa: ['Telegram را با bot token و chat id تنظیم کنید.', 'eventها را انتخاب کنید.', 'VirusTotal API key را تنظیم کنید.', 'در Target module مربوطه را فعال کنید.'],
      en: ['Configure Telegram with bot token and chat id.', 'Select events.', 'Configure VirusTotal API key.', 'Enable the related target module.'],
    },
    outputs: {
      fa: ['notification config', 'ارسال eventها', 'VirusTotal-derived URLs/assets', 'source attribution'],
      en: ['notification config', 'event delivery', 'VirusTotal-derived URLs/assets', 'source attribution'],
    },
    errors: {
      fa: ['bot token نامعتبر', 'chat id اشتباه', 'bot دسترسی ندارد', 'VirusTotal API invalid', 'rate limit'],
      en: ['invalid bot token', 'wrong chat id', 'bot lacks access', 'invalid VirusTotal API', 'rate limit'],
    },
    security: {
      fa: ['bot token، chat id و API key را sanitize کنید. notificationها ممکن است target data حساس داشته باشند.'],
      en: ['Sanitize bot token, chat id, and API key. Notifications may contain sensitive target data.'],
    },
    screenshot: '/docs/screenshots/fa/settings-telegram-sanitized.png',
  },
  {
    id: 'settings-monitoring-logs',
    group: 'settings',
    icon: 'terminal',
    title: { fa: 'Monitoring Server و System Logs', en: 'Monitoring Server and System Logs' },
    route: '/settings → Monitoring Server / System Logs',
    purpose: {
      fa: 'بررسی وضعیت سیستم، processهای فعال، resource usage و logها برای debug.',
      en: 'Inspect system health, active processes, resource usage, and logs for debugging.',
    },
    features: {
      fa: ['CPU usage', 'memory usage', 'goroutines', 'active processes', 'PID', 'duration', 'command', 'logs', 'runtime errors'],
      en: ['CPU usage', 'memory usage', 'goroutines', 'active processes', 'PID', 'duration', 'command', 'logs', 'runtime errors'],
    },
    howToUse: {
      fa: ['Monitoring را برای وضعیت سیستم باز کنید.', 'processهای طولانی را بررسی کنید.', 'برای خطا System Logs را بخوانید.', 'خطا را با queue/target/job تطبیق دهید.'],
      en: ['Open Monitoring for system state.', 'Inspect long-running processes.', 'Read System Logs for errors.', 'Correlate error with queue/target/job.'],
    },
    outputs: {
      fa: ['system stats', 'process list', 'log lines', 'error context'],
      en: ['system stats', 'process list', 'log lines', 'error context'],
    },
    errors: {
      fa: ['process stale', 'logs noisy', 'sensitive data in logs', 'monitoring API unavailable'],
      en: ['stale process', 'noisy logs', 'sensitive data in logs', 'monitoring API unavailable'],
    },
    security: {
      fa: ['command/log ممکن است target، path یا secret داشته باشد؛ قبل از share کردن sanitize کنید.'],
      en: ['Commands/logs may contain targets, paths, or secrets; sanitize before sharing.'],
    },
    screenshot: '/docs/screenshots/fa/settings-monitoring.png',
  },
  {
    id: 'settings-feature-flags',
    group: 'settings',
    icon: 'settings',
    title: { fa: 'Feature Flags', en: 'Feature Flags' },
    route: '/settings یا /account → Feature Flags',
    purpose: {
      fa: 'کنترل فعال/غیرفعال بودن قابلیت‌ها در سطح global یا account با حالت inherit/enabled/disabled.',
      en: 'Control feature availability globally or per account using inherit/enabled/disabled states.',
    },
    features: {
      fa: ['feature.target_policy', 'feature.target_pdf_report', 'feature.ai_analysis', 'feature.llm_assisted_analysis', 'feature.ai_recommendations', 'feature.ai_nuclei_template_drafts', 'feature.agent_runs', 'feature.agent_actions', 'feature.agent_chat', 'feature.safe_bug_testing', 'feature.ai_triage_agent', 'feature.ai_summary_agent', 'feature.ai_report_agent'],
      en: ['feature.target_policy', 'feature.target_pdf_report', 'feature.ai_analysis', 'feature.llm_assisted_analysis', 'feature.ai_recommendations', 'feature.ai_nuclei_template_drafts', 'feature.agent_runs', 'feature.agent_actions', 'feature.agent_chat', 'feature.safe_bug_testing', 'feature.ai_triage_agent', 'feature.ai_summary_agent', 'feature.ai_report_agent'],
    },
    howToUse: {
      fa: ['Feature Flags را باز کنید.', 'effective state را بررسی کنید.', 'در صورت نیاز inherit/enabled/disabled را تغییر دهید.', 'اثر تغییر را در UI و backend workflow تست کنید.'],
      en: ['Open Feature Flags.', 'Review effective state.', 'Change inherit/enabled/disabled if needed.', 'Test the effect in UI and backend workflow.'],
    },
    outputs: {
      fa: ['effective feature state', 'نمایش/عدم نمایش tabها', 'فعال/غیرفعال شدن workflowها'],
      en: ['effective feature state', 'visible/hidden tabs', 'enabled/disabled workflows'],
    },
    errors: {
      fa: ['feature hidden', 'global flag با account override conflict دارد', 'کاربر انتظار tab دارد ولی flag disabled است'],
      en: ['feature hidden', 'global flag conflicts with account override', 'user expects a tab but flag is disabled'],
    },
    security: {
      fa: ['feature flags می‌توانند workflowهای حساس را فعال کنند؛ برای Operator/security features حتماً policy و authorization را بررسی کنید.'],
      en: ['Feature flags can enable sensitive workflows; review policy and authorization for Operator/security features.'],
    },
    screenshot: '/docs/screenshots/fa/settings-feature-flags.png',
  },

  {
    id: 'account-provider-settings',
    group: 'settings',
    icon: 'user',
    title: { fa: 'Account و Provider Settings', en: 'Account and Provider Settings' },
    route: '/account',
    purpose: {
      fa: 'مدیریت پروفایل کاربر، تغییر رمز، queue شخصی، provider keyها، LLM providerها، Telegram و feature flagهای account.',
      en: 'Manage user profile, password change, personal queue, provider keys, LLM providers, Telegram, and account feature flags.',
    },
    features: {
      fa: ['username', 'role', 'created date', 'concurrent scan slots', 'change password', 'my scan queue', 'Subfinder provider keys', 'LLM providers', 'Telegram config', 'account feature flags'],
      en: ['username', 'role', 'created date', 'concurrent scan slots', 'change password', 'my scan queue', 'Subfinder provider keys', 'LLM providers', 'Telegram config', 'account feature flags'],
    },
    howToUse: {
      fa: ['Account را باز کنید.', 'اطلاعات profile و scan slots را بررسی کنید.', 'برای تغییر رمز current و new password را وارد کنید.', 'provider keyها را با show/hide مدیریت کنید.', 'LLM/Telegram/feature flags را در صورت نیاز تنظیم کنید.'],
      en: ['Open Account.', 'Review profile and scan slots.', 'Enter current and new password to change password.', 'Manage provider keys with show/hide.', 'Configure LLM/Telegram/feature flags when needed.'],
    },
    outputs: {
      fa: ['profile data', 'password update status', 'provider config saved state', 'queue state', 'effective feature flags'],
      en: ['profile data', 'password update status', 'provider config saved state', 'queue state', 'effective feature flags'],
    },
    errors: {
      fa: ['current password incorrect', 'provider key invalid', 'LLM provider invalid', 'Telegram config invalid', 'feature flag override conflict'],
      en: ['current password incorrect', 'invalid provider key', 'invalid LLM provider', 'invalid Telegram config', 'feature flag override conflict'],
    },
    security: {
      fa: ['رمز، API key، bot token، chat id و secretها را در screenshot یا docs نمایش ندهید.'],
      en: ['Do not expose passwords, API keys, bot tokens, chat IDs, or secrets in screenshots or docs.'],
    },
    screenshot: '/docs/screenshots/fa/account-profile.png',
  },
  {
    id: 'nuclei-templates-workflow',
    group: 'nuclei',
    icon: 'file',
    title: { fa: 'Nuclei Templates Workflow', en: 'Nuclei Templates Workflow' },
    route: '/nuclei-templates',
    purpose: {
      fa: 'مدیریت templateها، placementها، validation، custom templateها و AI-assisted draft workflow.',
      en: 'Manage templates, placements, validation, custom templates, and AI-assisted draft workflow.',
    },
    features: {
      fa: ['Root', 'Shared', 'Safe', 'Fast', 'Exposure', 'Balanced', 'Misconfig', 'CVEs', 'CVEs Light', 'Full', 'Custom', 'Search', 'Create', 'Save', 'Validate', 'Delete', 'AI Draft', 'Strategy Signals'],
      en: ['Root', 'Shared', 'Safe', 'Fast', 'Exposure', 'Balanced', 'Misconfig', 'CVEs', 'CVEs Light', 'Full', 'Custom', 'Search', 'Create', 'Save', 'Validate', 'Delete', 'AI Draft', 'Strategy Signals'],
    },
    howToUse: {
      fa: ['Nuclei Templates را باز کنید.', 'placement مناسب را انتخاب کنید.', 'template را search یا ایجاد کنید.', 'YAML را edit کنید.', 'قبل از save، validate بزنید.', 'AI draftها را human-review کنید.'],
      en: ['Open Nuclei Templates.', 'Select the proper placement.', 'Search or create a template.', 'Edit YAML.', 'Validate before saving.', 'Human-review AI drafts.'],
    },
    outputs: {
      fa: ['template YAML', 'validation result', 'placement', 'strategy signals', 'generated draft'],
      en: ['template YAML', 'validation result', 'placement', 'strategy signals', 'generated draft'],
    },
    errors: {
      fa: ['YAML invalid', 'nuclei validation failed', 'placement اشتباه', 'AI draft disabled', 'provider/model missing'],
      en: ['invalid YAML', 'nuclei validation failed', 'wrong placement', 'AI draft disabled', 'provider/model missing'],
    },
    security: {
      fa: ['Template جدید نباید destructive یا out-of-scope باشد. اجرای خودکار نیازمند authorization و policy است.'],
      en: ['New templates must not be destructive or out-of-scope. Auto-execution requires authorization and policy.'],
    },
    screenshot: '/docs/screenshots/fa/nuclei-templates-list.png',
  },
  {
    id: 'reports-and-exports',
    group: 'target-detail',
    icon: 'database',
    title: { fa: 'Reports و Export Actions', en: 'Reports and Export Actions' },
    route: '/targets, /targets/:id, Findings',
    purpose: {
      fa: 'خروجی گرفتن برای backup، migration، review آفلاین، گزارش PDF و export شواهد.',
      en: 'Export data for backup, migration, offline review, PDF reports, and evidence sharing.',
    },
    features: {
      fa: ['PDF Report', 'Export Targets', 'Export Assets', 'Export IPs', 'Export URLs', 'Export Findings CSV', 'Export Findings JSON'],
      en: ['PDF Report', 'Export Targets', 'Export Assets', 'Export IPs', 'Export URLs', 'Export Findings CSV', 'Export Findings JSON'],
    },
    howToUse: {
      fa: ['قبل از export فیلترها را بررسی کنید.', 'Export مناسب را انتخاب کنید.', 'فایل را امن ذخیره کنید.', 'قبل از share کردن sanitize کنید.'],
      en: ['Review active filters before export.', 'Choose the relevant export.', 'Store the file securely.', 'Sanitize before sharing.'],
    },
    outputs: {
      fa: ['PDF', 'JSON', 'CSV', 'TXT', 'filtered export files'],
      en: ['PDF', 'JSON', 'CSV', 'TXT', 'filtered export files'],
    },
    errors: {
      fa: ['export خالی به دلیل filter', 'feature flag disabled', 'download blocked', 'schema/version mismatch'],
      en: ['empty export due to filters', 'feature flag disabled', 'download blocked', 'schema/version mismatch'],
    },
    security: {
      fa: ['Exportها شامل attack surface و evidence حساس هستند. آن‌ها را public commit نکنید.'],
      en: ['Exports contain sensitive attack-surface and evidence data. Do not commit them publicly.'],
    },
    screenshot: '/docs/screenshots/fa/export-assets.png',
  },
  {
    id: 'troubleshooting-frontend',
    group: 'support',
    icon: 'terminal',
    title: { fa: 'Troubleshooting: Frontend Routes', en: 'Troubleshooting: Frontend Routes' },
    route: 'Stage/Production frontend routes',
    purpose: {
      fa: 'عیب‌یابی routeهای React SPA مثل /documentation، /login، /dashboard و routeهای protected.',
      en: 'Troubleshoot React SPA routes such as /documentation, /login, /dashboard, and protected routes.',
    },
    features: {
      fa: ['HTTP status', 'React shell', 'JS asset bundle', 'bundle markers', 'docker cp dist', 'nginx restart', 'hard refresh'],
      en: ['HTTP status', 'React shell', 'JS asset bundle', 'bundle markers', 'docker cp dist', 'nginx restart', 'hard refresh'],
    },
    howToUse: {
      fa: ['status code را با curl بگیرید.', 'وجود div root را بررسی کنید.', 'JS asset را پیدا کنید.', 'bundle marker را grep کنید.', 'در dev dist را داخل container کپی کنید و nginx را restart کنید.', 'browser hard refresh کنید.'],
      en: ['Check status code with curl.', 'Verify div root.', 'Find JS asset.', 'grep bundle markers.', 'Copy dist into container and restart nginx in dev.', 'Hard refresh browser.'],
    },
    outputs: {
      fa: ['HTTP 200', 'React shell', 'asset path', 'marker match', 'route working'],
      en: ['HTTP 200', 'React shell', 'asset path', 'marker match', 'route working'],
    },
    errors: {
      fa: ['404 route', 'stale asset', 'nginx cache', 'old frontend container content', 'browser cache'],
      en: ['404 route', 'stale asset', 'nginx cache', 'old frontend container content', 'browser cache'],
    },
    security: {
      fa: ['در خروجی debug توکن یا اطلاعات حساس paste نکنید.'],
      en: ['Do not paste tokens or sensitive data in debug output.'],
    },
    screenshot: '/docs/screenshots/fa/troubleshooting-frontend.png',
  },
  {
    id: 'troubleshooting-backend-api',
    group: 'support',
    icon: 'terminal',
    title: { fa: 'Troubleshooting: Backend و API', en: 'Troubleshooting: Backend and API' },
    route: '/api/*',
    purpose: {
      fa: 'عیب‌یابی API، auth، role، backend reload، DB migration و handler/runtime errorها.',
      en: 'Troubleshoot API, auth, role, backend reload, DB migrations, and handler/runtime errors.',
    },
    features: {
      fa: ['backend container', 'API status', 'JWT', 'role permission', 'logs', 'DB connection', 'migrations', 'fast backend reload'],
      en: ['backend container', 'API status', 'JWT', 'role permission', 'logs', 'DB connection', 'migrations', 'fast backend reload'],
    },
    howToUse: {
      fa: ['API endpoint را با auth تست کنید.', 'backend logs را بررسی کنید.', 'DB migrationها را چک کنید.', 'در dev از fast backend reload استفاده کنید.', 'role و feature flag را بررسی کنید.'],
      en: ['Test API endpoint with auth.', 'Inspect backend logs.', 'Check DB migrations.', 'Use fast backend reload in dev.', 'Review role and feature flag.'],
    },
    outputs: {
      fa: ['API response', 'error context', 'backend log', 'migration state'],
      en: ['API response', 'error context', 'backend log', 'migration state'],
    },
    errors: {
      fa: ['401 token', '403 permission', '500 handler error', 'DB migration mismatch', 'env missing'],
      en: ['401 token', '403 permission', '500 handler error', 'DB migration mismatch', 'missing env'],
    },
    security: {
      fa: ['JWT، secrets و env valueهای حساس را در docs یا support paste نکنید.'],
      en: ['Do not paste JWTs, secrets, or sensitive env values in docs/support.'],
    },
    screenshot: '/docs/screenshots/fa/troubleshooting-backend-api.png',
  },
  {
    id: 'troubleshooting-recon',
    group: 'support',
    icon: 'terminal',
    title: { fa: 'Troubleshooting: Recon و DNS', en: 'Troubleshooting: Recon and DNS' },
    route: 'Targets / Active Processes / Settings',
    purpose: {
      fa: 'عیب‌یابی discovery job، DNSX، PureDNS، resolverها، wildcard DNS، AlterX و URL sources.',
      en: 'Troubleshoot discovery jobs, DNSX, PureDNS, resolvers, wildcard DNS, AlterX, and URL sources.',
    },
    features: {
      fa: ['Active Processes', 'Queue Manager', 'PureDNS progress', 'resolver pool', 'wordlist status', 'wildcard filtering', 'DNSX validation', 'AlterX validation'],
      en: ['Active Processes', 'Queue Manager', 'PureDNS progress', 'resolver pool', 'wordlist status', 'wildcard filtering', 'DNSX validation', 'AlterX validation'],
    },
    howToUse: {
      fa: ['Active Processes را بررسی کنید.', 'Queue را چک کنید.', 'PureDNS rate/ETA را بخوانید.', 'resolverها و wordlist را بررسی کنید.', 'wildcard filtering و DNSX live parsing را validate کنید.'],
      en: ['Check Active Processes.', 'Inspect Queue.', 'Read PureDNS rate/ETA.', 'Review resolvers and wordlists.', 'Validate wildcard filtering and DNSX live parsing.'],
    },
    outputs: {
      fa: ['علت کندی یا noise', 'resolver bottleneck', 'wildcard issue', 'module/source issue'],
      en: ['cause of slowness/noise', 'resolver bottleneck', 'wildcard issue', 'module/source issue'],
    },
    errors: {
      fa: ['PureDNS کند', 'resolver throttle', 'wordlist عظیم', 'wildcard noise', 'false live', 'URL source empty'],
      en: ['slow PureDNS', 'resolver throttle', 'huge wordlist', 'wildcard noise', 'false live', 'empty URL source'],
    },
    security: {
      fa: ['brute-force و active crawl باید scope-aware، rate-aware و مجاز باشند.'],
      en: ['brute-force and active crawl must be scope-aware, rate-aware, and authorized.'],
    },
    screenshot: '/docs/screenshots/fa/troubleshooting-recon.png',
  },
  {
    id: 'troubleshooting-operator',
    group: 'support',
    icon: 'brain',
    title: { fa: 'Troubleshooting: AI Operator', en: 'Troubleshooting: AI Operator' },
    route: '/targets/:id → Analysis → Attack Surface Chat',
    purpose: {
      fa: 'عیب‌یابی خروجی generic، skill not implemented، policy blocked، approval required، inconclusive و missing context.',
      en: 'Troubleshoot generic output, skill not implemented, policy blocked, approval required, inconclusive, and missing context states.',
    },
    features: {
      fa: ['selected_skills', 'skill_execution', 'not_implemented', 'policy blocked', 'approval required', 'inconclusive', 'methodology context', 'memory learning'],
      en: ['selected_skills', 'skill_execution', 'not_implemented', 'policy blocked', 'approval required', 'inconclusive', 'methodology context', 'memory learning'],
    },
    howToUse: {
      fa: ['Target evidence را بررسی کنید.', 'Skill Profile را چک کنید.', 'فعال بودن skill و methodology را بررسی کنید.', 'Policy را ببینید.', 'output_json و observations را بخوانید.', 'برای auth context اطلاعات لازم را فراهم کنید.'],
      en: ['Review target evidence.', 'Check Skill Profile.', 'Verify skill and methodology are enabled.', 'Review Policy.', 'Read output_json and observations.', 'Provide auth context when needed.'],
    },
    outputs: {
      fa: ['diagnosis', 'next step', 'missing evidence/context', 'policy or runtime reason'],
      en: ['diagnosis', 'next step', 'missing evidence/context', 'policy or runtime reason'],
    },
    errors: {
      fa: ['LLM provider missing', 'no candidates', 'skill disabled', 'runtime backend blocked', 'policy blocked', 'auth context missing'],
      en: ['LLM provider missing', 'no candidates', 'skill disabled', 'runtime backend blocked', 'policy blocked', 'auth context missing'],
    },
    security: {
      fa: ['برای رفع policy blocked، policy را کورکورانه باز نکنید؛ authorization واقعی را بررسی کنید.'],
      en: ['Do not blindly loosen policy to fix blocked actions; verify real authorization.'],
    },
    screenshot: '/docs/screenshots/fa/troubleshooting-operator.png',
  },
  {
    id: 'troubleshooting-deployment',
    group: 'support',
    icon: 'terminal',
    title: { fa: 'Troubleshooting: Deployment', en: 'Troubleshooting: Deployment' },
    route: 'Dev/Stage/Production deployment',
    purpose: {
      fa: 'عیب‌یابی deploy در dev/stage/production، frontend bundle، nginx، Cloudflare، SSL و container recreate.',
      en: 'Troubleshoot dev/stage/production deploys, frontend bundle, nginx, Cloudflare, SSL, and container recreation.',
    },
    features: {
      fa: ['dev path', 'prod path', 'source-based compose', 'frontend build', 'docker cp dist', 'nginx restart', 'Cloudflare origin lock', 'SSL', 'route smoke'],
      en: ['dev path', 'prod path', 'source-based compose', 'frontend build', 'docker cp dist', 'nginx restart', 'Cloudflare origin lock', 'SSL', 'route smoke'],
    },
    howToUse: {
      fa: ['در dev از /opt/hunt-engine/dev/app استفاده کنید.', 'frontend را build کنید.', 'dist را داخل container کپی کنید.', 'nginx را restart کنید.', 'route smoke بزنید.', 'برای production مسیر /opt/hunt-engine/prod/app را رعایت کنید.'],
      en: ['Use /opt/hunt-engine/dev/app in dev.', 'Build frontend.', 'Copy dist into the container.', 'Restart nginx.', 'Run route smoke.', 'Use /opt/hunt-engine/prod/app for production.'],
    },
    outputs: {
      fa: ['deploy سالم', 'route 200', 'bundle marker match', 'nginx/SSL درست'],
      en: ['healthy deploy', 'route 200', 'bundle marker match', 'correct nginx/SSL'],
    },
    errors: {
      fa: ['stale frontend', 'nginx cached old upstream', 'Cloudflare mismatch', 'direct IP blocked intentionally', 'Node version warning'],
      en: ['stale frontend', 'nginx cached old upstream', 'Cloudflare mismatch', 'direct IP intentionally blocked', 'Node version warning'],
    },
    security: {
      fa: ['direct IP/origin lock باید حفظ شود. production secrets و env را در docs paste نکنید.'],
      en: ['direct IP/origin lock should remain enforced. Do not paste production secrets/env in docs.'],
    },
    screenshot: '/docs/screenshots/fa/troubleshooting-deployment.png',
  },

];

const groupLabels: Record<string, { fa: string; en: string }> = {
  access: { fa: 'دسترسی', en: 'Access' },
  dashboard: { fa: 'داشبورد', en: 'Dashboard' },
  targets: { fa: 'تارگت‌ها', en: 'Targets' },
  'target-detail': { fa: 'جزئیات Target', en: 'Target Detail' },
  operator: { fa: 'AI Operator', en: 'AI Operator' },
  nuclei: { fa: 'Nuclei', en: 'Nuclei' },
  settings: { fa: 'تنظیمات', en: 'Settings' },
  support: { fa: 'عیب‌یابی', en: 'Support' },
};

const groupOrder = ['access', 'dashboard', 'targets', 'target-detail', 'operator', 'nuclei', 'settings', 'support'];

const icons = {
  lock: Lock,
  dashboard: LayoutDashboard,
  globe: Globe,
  database: Database,
  search: Search,
  shield: Shield,
  brain: BrainCircuit,
  terminal: TerminalSquare,
  file: FileCode2,
  user: User2,
  settings: Settings,
  filter: Filter,
};

function t(lang: DocsLang, value: { fa: string; en: string }) {
  return value[lang];
}

export default function Documentation() {
  const [lang, setLang] = useState<DocsLang>('fa');
  const [activeId, setActiveId] = useState('target-detail-assets');
  const [query, setQuery] = useState('');

  const active = useMemo(
    () => featureDocs.find((item) => item.id === activeId) ?? featureDocs[0],
    [activeId],
  );

  const filteredDocs = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return featureDocs;
    return featureDocs.filter((item) => {
      const haystack = [
        item.id,
        item.group,
        item.route,
        item.title.fa,
        item.title.en,
        item.purpose.fa,
        item.purpose.en,
        ...item.features.fa,
        ...item.features.en,
      ]
        .join(' ')
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [query]);

  const dir = lang === 'fa' ? 'rtl' : 'ltr';

  return (
    <div
      dir={dir}
      className="min-h-screen bg-hack-bg bg-grid-pattern bg-[size:40px_40px] text-hack-text"
      style={{
        fontFamily:
          lang === 'fa'
            ? 'Vazirmatn, Tahoma, Arial, sans-serif'
            : '"Fira Code", "JetBrains Mono", monospace',
      }}
    >
      <div className="fixed inset-0 pointer-events-none bg-[linear-gradient(rgba(18,16,16,0)_50%,rgba(0,0,0,0.1)_50%),linear-gradient(90deg,rgba(255,0,0,0.06),rgba(0,255,0,0.02),rgba(0,0,255,0.06))] bg-[length:100%_2px,3px_100%] opacity-20" />

      <div className="relative z-10 flex min-h-screen">
        <aside className="hidden w-72 shrink-0 border-r border-hack-primary/20 bg-hack-panel/90 backdrop-blur-md lg:flex lg:flex-col">
          <div className="border-b border-hack-border/50 p-5 text-center">
            <div className="mx-auto mb-3 w-28 opacity-90">
              <MustacheLogo />
            </div>
            <div className="font-display text-2xl tracking-widest text-hack-primary">
              HUNT<span className="text-white">OS</span> DOCS
            </div>
            <div className="mt-2 flex items-center justify-center gap-2 text-[10px] uppercase tracking-[0.2em] text-hack-dim">
              <span className="h-2 w-2 rounded-full bg-hack-primary" />
              v3.15.2
            </div>
          </div>

          <nav className="flex-1 overflow-y-auto p-4">
            <div className="mb-3 border-b border-hack-border/30 px-2 py-2 text-[10px] uppercase tracking-widest text-hack-dim">
              Documentation Map
            </div>

            <label className="mb-4 block">
              <div className="relative">
                <Search className="absolute left-3 top-2.5 h-4 w-4 text-hack-dim" />
                <input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={lang === 'fa' ? 'جستجو در مستندات...' : 'Search docs...'}
                  className="w-full border border-hack-border bg-black/40 py-2 pl-9 pr-3 text-sm text-hack-primary outline-none placeholder:text-hack-dim/60 focus:border-hack-primary"
                  dir={lang === 'fa' ? 'rtl' : 'ltr'}
                />
              </div>
            </label>

            <div className="space-y-5">
              {groupOrder.map((group) => {
                const items = filteredDocs.filter((item) => item.group === group);
                if (!items.length) return null;
                return (
                  <div key={group}>
                    <div className="mb-2 px-2 text-[10px] font-bold uppercase tracking-widest text-hack-dim">
                      {groupLabels[group][lang]}
                    </div>
                    <div className="space-y-1">
                      {items.map((item) => {
                        const Icon = icons[item.icon as keyof typeof icons] ?? BookOpen;
                        const isActive = active.id === item.id;
                        return (
                          <button
                            key={item.id}
                            type="button"
                            onClick={() => setActiveId(item.id)}
                            className={clsx(
                              'w-full border px-3 py-2.5 text-start transition-all',
                              isActive
                                ? 'border-hack-primary/60 bg-hack-primary/10 text-hack-primary shadow-[0_0_10px_rgba(0,255,65,0.1)]'
                                : 'border-transparent text-hack-dim hover:bg-white/5 hover:text-hack-text',
                            )}
                          >
                            <span className="flex items-center gap-2 text-sm font-semibold">
                              <Icon className="h-4 w-4 shrink-0" />
                              <span>{t(lang, item.title)}</span>
                            </span>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                );
              })}
            </div>
          </nav>

          <div className="border-t border-hack-border/50 p-4">
            <Link
              to="/login"
              className="flex w-full items-center justify-center gap-2 border border-hack-primary/40 bg-hack-primary/5 px-4 py-3 text-xs font-bold uppercase tracking-widest text-hack-primary transition-all hover:bg-hack-primary hover:text-black"
            >
              Open App
            </Link>
          </div>
        </aside>

        <main className="flex-1 overflow-auto">
          <div className="mx-auto max-w-[1800px] p-4 md:p-8">
            <header className="mb-6 flex flex-col gap-4 border-b border-hack-border/60 pb-5 md:flex-row md:items-center md:justify-between">
              <div>
                <div className="mb-2 flex items-center gap-2 text-[10px] uppercase tracking-widest text-hack-dim">
                  <span className="text-hack-primary">Documentation Portal</span>
                  <span>/</span>
                  <span>Feature-by-feature User Guide</span>
                </div>
                <h1
                  className={clsx(
                    'text-2xl font-bold text-white md:text-3xl',
                    lang === 'en' && 'font-mono uppercase tracking-wider',
                  )}
                >
                  {lang === 'fa'
                    ? 'راهنمای نقطه‌به‌نقطه Hunt Engine'
                    : 'Hunt Engine Feature-by-Feature Guide'}
                </h1>
                <p className="mt-3 max-w-4xl text-sm leading-7 text-hack-dim md:text-base">
                  {lang === 'fa'
                    ? 'این مستندات باید تمام صفحه‌ها، تب‌ها، دکمه‌ها، تنظیمات، runtimeها، خروجی‌ها، خطاها و مرزهای authorization پروژه را آموزش دهد. هر قابلیت جدید یا تغییر یافته باید همین‌جا مستند شود.'
                    : 'These docs must cover every page, tab, button, setting, runtime, output, error, and authorization boundary. Every new or changed feature must update this portal.'}
                </p>
              </div>

              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => setLang('fa')}
                  className={clsx(
                    'border px-4 py-2 text-xs font-bold transition-all',
                    lang === 'fa'
                      ? 'border-hack-primary bg-hack-primary text-black'
                      : 'border-hack-border text-hack-dim hover:text-hack-primary',
                  )}
                >
                  فارسی
                </button>
                <button
                  type="button"
                  onClick={() => setLang('en')}
                  className={clsx(
                    'border px-4 py-2 text-xs font-bold uppercase tracking-widest transition-all',
                    lang === 'en'
                      ? 'border-hack-primary bg-hack-primary text-black'
                      : 'border-hack-border text-hack-dim hover:text-hack-primary',
                  )}
                >
                  English
                </button>
                <Link to="/" className="hack-btn-ghost border border-hack-border px-4 py-2">
                  Home
                </Link>
              </div>
            </header>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_380px]">
              <section className="space-y-5">
                <article className="hack-box p-5 md:p-6">
                  <div className="mb-5 flex flex-col gap-3 border-b border-hack-border/50 pb-5 md:flex-row md:items-start md:justify-between">
                    <div>
                      <div className="mb-2 text-[10px] font-bold uppercase tracking-widest text-hack-primary">
                        {groupLabels[active.group][lang]}
                      </div>
                      <h2 className="text-xl font-bold text-white md:text-2xl">{t(lang, active.title)}</h2>
                      <p className="mt-3 max-w-4xl text-sm leading-7 text-hack-text/85">{t(lang, active.purpose)}</p>
                    </div>

                    <div className="border border-hack-border bg-black/40 px-4 py-3 text-xs text-hack-dim md:min-w-72">
                      <div className="mb-1 text-[10px] uppercase tracking-widest text-hack-primary">
                        {lang === 'fa' ? 'مسیر UI' : 'UI Path'}
                      </div>
                      <code className="text-hack-text">{active.route}</code>
                    </div>
                  </div>

                  <DocBlock
                    lang={lang}
                    titleFa="قابلیت‌ها و کنترل‌ها"
                    titleEn="Features and controls"
                    items={active.features[lang]}
                  />

                  <DocBlock
                    lang={lang}
                    titleFa="نحوه استفاده"
                    titleEn="How to use"
                    items={active.howToUse[lang]}
                    ordered
                  />

                  <DocBlock
                    lang={lang}
                    titleFa="خروجی‌ها و نحوه تفسیر"
                    titleEn="Outputs and interpretation"
                    items={active.outputs[lang]}
                  />

                  <DocBlock
                    lang={lang}
                    titleFa="خطاهای رایج"
                    titleEn="Common errors"
                    items={active.errors[lang]}
                  />

                  <DocBlock
                    lang={lang}
                    titleFa="محدوده امنیتی و authorization"
                    titleEn="Security and authorization boundaries"
                    items={active.security[lang]}
                    danger
                  />
                </article>
              </section>

              <aside className="space-y-5">
                <div className="hack-box p-5">
                  <div className="mb-3 text-[10px] font-bold uppercase tracking-widest text-hack-primary">
                    Screenshot Required
                  </div>
                  <div className="border border-dashed border-hack-primary/30 bg-black/40 p-4">
                    <div className="text-sm font-bold text-white">
                      {lang === 'fa' ? 'اسکرین‌شات واقعی UI' : 'Real UI screenshot'}
                    </div>
                    <p className="mt-2 text-xs leading-6 text-hack-dim">
                      {lang === 'fa'
                        ? 'اگر فایل screenshot در مسیر زیر وجود داشته باشد، همین‌جا نمایش داده می‌شود.'
                        : 'If the screenshot file exists at the path below, it is displayed here.'}
                    </p>

                    <div className="mt-4 overflow-hidden border border-hack-border bg-hack-bg">
                      <img
                        src={active.screenshot}
                        alt={active.title[lang]}
                        className="max-h-72 w-full object-contain"
                        onError={(event) => {
                          event.currentTarget.style.display = 'none';
                        }}
                      />
                    </div>

                    <code className="mt-3 block break-all border border-hack-border bg-hack-bg p-3 text-[11px] leading-5 text-hack-primary">
                      {active.screenshot}
                    </code>
                  </div>
                </div>

                <div className="hack-box p-5">
                  <div className="mb-3 text-[10px] font-bold uppercase tracking-widest text-hack-primary">
                    Documentation DoD
                  </div>
                  <ul className="space-y-2 text-xs leading-6 text-hack-dim">
                    <li>• {lang === 'fa' ? 'آموزش فارسی و انگلیسی' : 'Persian and English guide'}</li>
                    <li>• {lang === 'fa' ? 'مسیر UI و مراحل استفاده' : 'UI path and usage steps'}</li>
                    <li>• {lang === 'fa' ? 'خروجی‌ها، خطاها و تفسیر evidence' : 'Outputs, errors, and evidence interpretation'}</li>
                    <li>• {lang === 'fa' ? 'اسکرین‌شات sanitized' : 'Sanitized screenshot'}</li>
                    <li>• {lang === 'fa' ? 'مرزهای scope و authorization' : 'Scope and authorization boundaries'}</li>
                  </ul>
                </div>

                <div className="hack-box p-5">
                  <div className="mb-3 text-[10px] font-bold uppercase tracking-widest text-hack-primary">
                    Current Coverage
                  </div>
                  <div className="grid grid-cols-2 gap-2 text-xs">
                    <Metric label="Pages" value="11" />
                    <Metric label="Components" value="29" />
                    <Metric label="API Modules" value="10" />
                    <Metric label="Doc Topics" value={String(featureDocs.length)} />
                  </div>
                </div>
              </aside>
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}

function DocBlock({
  lang,
  titleFa,
  titleEn,
  items,
  ordered = false,
  danger = false,
}: {
  lang: DocsLang;
  titleFa: string;
  titleEn: string;
  items: string[];
  ordered?: boolean;
  danger?: boolean;
}) {
  const Tag = ordered ? 'ol' : 'ul';

  return (
    <section className="mt-6">
      <h3 className="mb-3 text-sm font-bold text-hack-primary">
        {lang === 'fa' ? titleFa : titleEn}
      </h3>
      <Tag className="grid gap-2">
        {items.map((item, index) => (
          <li
            key={`${item}-${index}`}
            className={clsx(
              'border bg-black/30 px-4 py-3 text-sm leading-7',
              danger ? 'border-hack-danger/30 text-hack-danger/90' : 'border-hack-border text-hack-text',
            )}
          >
            {ordered && (
              <span className="mx-2 inline-flex h-5 w-5 items-center justify-center border border-hack-primary/50 text-[10px] text-hack-primary">
                {index + 1}
              </span>
            )}
            {item}
          </li>
        ))}
      </Tag>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-hack-border bg-black/30 p-3">
      <div className="text-[10px] uppercase tracking-widest text-hack-dim">{label}</div>
      <div className="mt-1 font-mono text-lg font-bold text-hack-primary">{value}</div>
    </div>
  );
}
