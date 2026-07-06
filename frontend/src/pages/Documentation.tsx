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
