import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import MustacheLogo from '../components/MustacheLogo';

type DocsLang = 'fa' | 'en';

type DocsSection = {
  id: string;
  title: {
    fa: string;
    en: string;
  };
  subtitle: {
    fa: string;
    en: string;
  };
  body: {
    fa: string[];
    en: string[];
  };
  steps?: {
    fa: string[];
    en: string[];
  };
  screenshot?: string;
};

const docsSections: DocsSection[] = [
  {
    id: 'quickstart',
    title: {
      fa: 'شروع سریع',
      en: 'Quickstart',
    },
    subtitle: {
      fa: 'مسیر پیشنهادی برای اولین استفاده از Hunt Engine، از ورود تا اولین تحلیل سطح حمله.',
      en: 'The recommended first-use path, from login to the first attack-surface review.',
    },
    body: {
      fa: [
        'Hunt Engine یک پلتفرم AI-assisted برای Attack Surface Intelligence، Recon، Evidence Review، Operator Skills و Controlled Authorized Validation است.',
        'برای شروع، ابتدا وارد حساب کاربری شوید، یک Target بسازید، Scope و Policy را تنظیم کنید، Recon را اجرا کنید و سپس نتایج را در Live Assets، URLs، Findings و Operator Chat بررسی کنید.',
      ],
      en: [
        'Hunt Engine is an AI-assisted platform for attack-surface intelligence, recon, evidence review, operator skills, and controlled authorized validation.',
        'Start by logging in, creating a target, configuring scope and policy, running recon, and reviewing results in Live Assets, URLs, Findings, and Operator Chat.',
      ],
    },
    steps: {
      fa: [
        'وارد /login شوید.',
        'از Dashboard یا Targets یک Target جدید بسازید.',
        'Target Policy، scope، rate limits و authorization boundaries را بررسی کنید.',
        'Discovery/Recon را اجرا کنید.',
        'Live Assets، URL inventory، JS Intelligence و Findings را بررسی کنید.',
        'از AI Operator برای پیشنهاد next step و اجرای controlled validation مجاز استفاده کنید.',
      ],
      en: [
        'Open /login.',
        'Create a new target from Dashboard or Targets.',
        'Review target policy, scope, rate limits, and authorization boundaries.',
        'Run discovery/recon.',
        'Review Live Assets, URL inventory, JS Intelligence, and Findings.',
        'Use the AI Operator for next-step reasoning and authorized controlled validation.',
      ],
    },
    screenshot: '/docs/screenshots/fa/quickstart-targets.png',
  },
  {
    id: 'recon-workflow',
    title: {
      fa: 'Recon از صفر تا صد',
      en: 'Recon End-to-End',
    },
    subtitle: {
      fa: 'جریان کامل recon شامل subdomain discovery، DNS validation، live detection، URL collection و evidence review.',
      en: 'The complete recon flow: subdomain discovery, DNS validation, live detection, URL collection, and evidence review.',
    },
    body: {
      fa: [
        'Recon در Hunt Engine باید به‌صورت مرحله‌ای خوانده شود: ابتدا discovery، سپس validation، سپس live asset review و در نهایت URL/JS/evidence analysis.',
        'PureDNS، DNSX و AlterX هرکدام نقش جدا دارند. خروجی هر مرحله باید با evidence و source قابل بررسی باشد.',
      ],
      en: [
        'Recon in Hunt Engine should be read as a staged workflow: discovery, validation, live asset review, and URL/JS/evidence analysis.',
        'PureDNS, DNSX, and AlterX have separate roles. Every stage should produce reviewable evidence and source attribution.',
      ],
    },
    steps: {
      fa: [
        'Target را باز کنید.',
        'Discovery profile مناسب را انتخاب کنید.',
        'Resolver و wordlist را در صورت نیاز تنظیم کنید.',
        'Active Processes و progress را در حین اجرا بررسی کنید.',
        'بعد از پایان، Live Assets و Sources را بررسی کنید.',
      ],
      en: [
        'Open the target.',
        'Choose the appropriate discovery profile.',
        'Configure resolvers and wordlists when needed.',
        'Monitor Active Processes and progress while the job runs.',
        'Review Live Assets and Sources when the job completes.',
      ],
    },
    screenshot: '/docs/screenshots/fa/recon-workflow.png',
  },
  {
    id: 'target-policy',
    title: {
      fa: 'Target Policy و محدوده مجاز',
      en: 'Target Policy and Authorized Scope',
    },
    subtitle: {
      fa: 'Policy مشخص می‌کند Operator چه چیزی را می‌تواند پیشنهاد، اجرا یا برای approval نگه دارد.',
      en: 'Policy controls what the Operator can propose, execute, or hold for approval.',
    },
    body: {
      fa: [
        'Target Policy مرز حرفه‌ای اجرای مجاز است، نه مانع تست واقعی. هر action باید scope-aware، policy-gated، audited و rate-limited باشد.',
        'برای تست‌های فعال‌تر، Autopilot mode، approval level، rate limit و stop conditions باید روشن و قابل audit باشند.',
      ],
      en: [
        'Target Policy is the professional boundary for authorized execution, not a blocker for real testing. Every action must be scope-aware, policy-gated, audited, and rate-limited.',
        'For more active tests, Autopilot mode, approval level, rate limit, and stop conditions must be explicit and auditable.',
      ],
    },
    screenshot: '/docs/screenshots/fa/target-policy.png',
  },
  {
    id: 'operator-chat',
    title: {
      fa: 'AI Operator Chat',
      en: 'AI Operator Chat',
    },
    subtitle: {
      fa: 'Operator شواهد target، حافظه، policy و skillها را ترکیب می‌کند تا next step تست را پیشنهاد یا اجرا کند.',
      en: 'The Operator combines target evidence, memory, policy, and skills to propose or execute the next testing step.',
    },
    body: {
      fa: [
        'Operator نباید فقط scanner passive باشد. هدف آن reasoning، hypothesis، controlled validation و evidence-driven finding promotion است.',
        'برای هر action، خروجی باید نشان دهد چه skillهایی انتخاب شدند، کدام اجرا شدند، چه evidence ساخته شد و چه چیزی inconclusive یا blocked بود.',
      ],
      en: [
        'The Operator is not intended to be a passive-only scanner. Its role is reasoning, hypothesis generation, controlled validation, and evidence-driven finding promotion.',
        'For every action, the output should show selected skills, executed skills, evidence, and blocked or inconclusive results.',
      ],
    },
    screenshot: '/docs/screenshots/fa/operator-chat.png',
  },
  {
    id: 'operator-skills',
    title: {
      fa: 'Operator Skills و Methodology',
      en: 'Operator Skills and Methodology',
    },
    subtitle: {
      fa: 'Executable Skills، user-defined skills و learning/methodology records مسیر انتخاب و اجرای تست را هدایت می‌کنند.',
      en: 'Executable skills, user-defined skills, and learning/methodology records guide test selection and execution.',
    },
    body: {
      fa: [
        'Executable Skills runtime-capable هستند. Methodology records دستورالعمل و تجربه کاربر را برای انتخاب و اجرای بهتر skillها فراهم می‌کنند.',
        'در Target Skill Profile می‌توان skillهای فعال و methodologyهای ترجیحی target را تنظیم کرد.',
      ],
      en: [
        'Executable Skills are runtime-capable. Methodology records provide user experience and testing guidance that influence skill selection and execution.',
        'Target Skill Profile can configure enabled skills and preferred methodologies for each target.',
      ],
    },
    screenshot: '/docs/screenshots/fa/operator-skills.png',
  },
  {
    id: 'bug-class-validation',
    title: {
      fa: 'Bug-Class Validation',
      en: 'Bug-Class Validation',
    },
    subtitle: {
      fa: 'v3.15.1 validation runtimes برای XSS، DOM XSS، CRLF، cache، open redirect، path traversal و CORS/CJ/CSRF.',
      en: 'v3.15.1 validation runtimes for XSS, DOM XSS, CRLF, cache, open redirect, path traversal, and CORS/CJ/CSRF.',
    },
    body: {
      fa: [
        'این runtimeها controlled Level 2 evidence می‌سازند: marker probes، header evidence، source/sink evidence و baseline behavior.',
        'این مرحله browser proof، sensitive file-read proof، raw CRLF payload، cache poisoning payload یا state-changing CSRF execution انجام نمی‌دهد.',
      ],
      en: [
        'These runtimes produce controlled Level 2 evidence: marker probes, header evidence, source/sink evidence, and baseline behavior.',
        'This stage does not perform browser proof, sensitive file-read proof, raw CRLF payloads, cache poisoning payloads, or state-changing CSRF execution.',
      ],
    },
    screenshot: '/docs/screenshots/fa/bug-class-validation.png',
  },
  {
    id: 'findings-evidence',
    title: {
      fa: 'Findings و Evidence',
      en: 'Findings and Evidence',
    },
    subtitle: {
      fa: 'یادگیری خواندن severity، confidence، evidence_json، screenshots، observations و status.',
      en: 'Learn how to read severity, confidence, evidence_json, screenshots, observations, and status.',
    },
    body: {
      fa: [
        'هر finding باید بر اساس evidence قابل بررسی باشد. پاسخ‌های 403/429/5xx، WAF و blocked evidence نباید به‌عنوان vulnerability قطعی claim شوند.',
        'هدف نهایی، promotion بر اساس evidence کیفیت‌دار، reproducibility و impact است.',
      ],
      en: [
        'Every finding must be based on reviewable evidence. 403/429/5xx, WAF, and blocked evidence must not be claimed as confirmed vulnerabilities.',
        'The end goal is promotion based on evidence quality, reproducibility, and impact.',
      ],
    },
    screenshot: '/docs/screenshots/fa/findings-evidence.png',
  },
  {
    id: 'admin-system',
    title: {
      fa: 'System Configuration و Admin',
      en: 'System Configuration and Admin',
    },
    subtitle: {
      fa: 'تنظیم کاربران، providerها، feature flagها، wordlistها، resolverها و سرویس‌ها.',
      en: 'Configure users, providers, feature flags, wordlists, resolvers, and services.',
    },
    body: {
      fa: [
        'Admin pages برای تنظیمات platform-level استفاده می‌شوند. تغییرات resolver، wordlist، LLM provider و feature flags می‌توانند روی تمام workflowها اثر بگذارند.',
      ],
      en: [
        'Admin pages control platform-level settings. Resolver, wordlist, LLM provider, and feature-flag changes can affect all workflows.',
      ],
    },
    screenshot: '/docs/screenshots/fa/admin-system.png',
  },
  {
    id: 'troubleshooting',
    title: {
      fa: 'Troubleshooting',
      en: 'Troubleshooting',
    },
    subtitle: {
      fa: 'راهنمای خطاهای رایج: login، route، recon jobs، DNS، PureDNS، API و frontend deploy.',
      en: 'Common issues: login, routes, recon jobs, DNS, PureDNS, API, and frontend deploy.',
    },
    body: {
      fa: [
        'برای خطاها ابتدا Active Processes، backend logs، nginx route behavior، API response و job status را بررسی کنید.',
        'هر خطای recurring باید به documentation و troubleshooting اضافه شود.',
      ],
      en: [
        'For errors, first check Active Processes, backend logs, nginx routing behavior, API response, and job status.',
        'Every recurring issue should be added to documentation and troubleshooting.',
      ],
    },
    screenshot: '/docs/screenshots/fa/troubleshooting.png',
  },
];

function text(lang: DocsLang, fa: string, en: string) {
  return lang === 'fa' ? fa : en;
}

export default function Documentation() {
  const [lang, setLang] = useState<DocsLang>('fa');
  const [activeId, setActiveId] = useState('quickstart');

  const activeSection = useMemo(() => {
    return docsSections.find((section) => section.id === activeId) ?? docsSections[0];
  }, [activeId]);

  const direction = lang === 'fa' ? 'rtl' : 'ltr';

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100" dir={direction}>
      <header className="border-b border-slate-800 bg-slate-950/95">
        <div className="mx-auto flex max-w-7xl flex-col gap-4 px-4 py-5 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 overflow-hidden rounded-xl"><MustacheLogo /></div>
            <div>
              <p className="text-xs uppercase tracking-[0.35em] text-cyan-300">Hunt Engine Docs</p>
              <h1 className="text-2xl font-bold text-white">
                {text(lang, 'مستندات رسمی Hunt Engine', 'Official Hunt Engine Documentation')}
              </h1>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              onClick={() => setLang('fa')}
              className={`rounded-full border px-4 py-2 text-sm font-semibold transition ${
                lang === 'fa'
                  ? 'border-cyan-300 bg-cyan-300 text-slate-950'
                  : 'border-slate-700 text-slate-200 hover:border-cyan-300'
              }`}
            >
              فارسی
            </button>
            <button
              type="button"
              onClick={() => setLang('en')}
              className={`rounded-full border px-4 py-2 text-sm font-semibold transition ${
                lang === 'en'
                  ? 'border-cyan-300 bg-cyan-300 text-slate-950'
                  : 'border-slate-700 text-slate-200 hover:border-cyan-300'
              }`}
            >
              English
            </button>
            <Link
              to="/"
              className="rounded-full border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:border-cyan-300"
            >
              {text(lang, 'بازگشت به صفحه اصلی', 'Back to home')}
            </Link>
          </div>
        </div>
      </header>

      <main className="mx-auto grid max-w-7xl gap-6 px-4 py-8 lg:grid-cols-[300px_minmax(0,1fr)]">
        <aside className="rounded-2xl border border-slate-800 bg-slate-900/70 p-4 lg:sticky lg:top-6 lg:h-[calc(100vh-3rem)] lg:overflow-y-auto">
          <p className="mb-3 text-xs font-semibold uppercase tracking-[0.25em] text-slate-400">
            {text(lang, 'فهرست', 'Contents')}
          </p>
          <nav className="space-y-2">
            {docsSections.map((section) => (
              <button
                key={section.id}
                type="button"
                onClick={() => setActiveId(section.id)}
                className={`w-full rounded-xl px-4 py-3 text-start text-sm transition ${
                  activeSection.id === section.id
                    ? 'bg-cyan-300 text-slate-950'
                    : 'bg-slate-950/60 text-slate-200 hover:bg-slate-800'
                }`}
              >
                <span className="block font-semibold">{section.title[lang]}</span>
                <span className={`mt-1 block text-xs ${activeSection.id === section.id ? 'text-slate-800' : 'text-slate-400'}`}>
                  {section.subtitle[lang]}
                </span>
              </button>
            ))}
          </nav>
        </aside>

        <section className="space-y-6">
          <div className="rounded-3xl border border-slate-800 bg-gradient-to-br from-slate-900 to-slate-950 p-6 shadow-2xl">
            <p className="mb-3 text-xs font-semibold uppercase tracking-[0.3em] text-cyan-300">
              {text(lang, 'Documentation Portal Foundation', 'Documentation Portal Foundation')}
            </p>
            <h2 className="text-3xl font-black text-white md:text-5xl">{activeSection.title[lang]}</h2>
            <p className="mt-4 max-w-3xl text-lg leading-8 text-slate-300">{activeSection.subtitle[lang]}</p>
          </div>

          <article className="rounded-3xl border border-slate-800 bg-slate-900/80 p-6">
            <div className="space-y-4 text-base leading-8 text-slate-200">
              {activeSection.body[lang].map((paragraph) => (
                <p key={paragraph}>{paragraph}</p>
              ))}
            </div>

            {activeSection.steps && (
              <div className="mt-8 rounded-2xl border border-slate-800 bg-slate-950/70 p-5">
                <h3 className="mb-4 text-xl font-bold text-white">
                  {text(lang, 'مراحل استفاده', 'How to use')}
                </h3>
                <ol className="space-y-3">
                  {activeSection.steps[lang].map((step, index) => (
                    <li key={step} className="flex gap-3 text-slate-200">
                      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-cyan-300 text-sm font-bold text-slate-950">
                        {index + 1}
                      </span>
                      <span className="leading-7">{step}</span>
                    </li>
                  ))}
                </ol>
              </div>
            )}

            <div className="mt-8 rounded-2xl border border-dashed border-slate-700 bg-slate-950/60 p-5">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <h3 className="text-lg font-bold text-white">
                    {text(lang, 'جایگاه اسکرین‌شات', 'Screenshot slot')}
                  </h3>
                  <p className="mt-2 text-sm leading-6 text-slate-400">
                    {text(
                      lang,
                      'برای هر feature باید تصویر UI واقعی در این مسیر قرار بگیرد و با تغییرات آینده آپدیت شود.',
                      'Each feature must include a real UI screenshot here and keep it updated with future changes.',
                    )}
                  </p>
                </div>
                <code className="rounded-xl bg-slate-900 px-3 py-2 text-xs text-cyan-200">
                  {activeSection.screenshot}
                </code>
              </div>
            </div>
          </article>

          <section className="grid gap-4 md:grid-cols-3">
            <div className="rounded-2xl border border-slate-800 bg-slate-900/70 p-5">
              <h3 className="font-bold text-white">{text(lang, 'الگوی هر صفحه', 'Page template')}</h3>
              <p className="mt-2 text-sm leading-6 text-slate-400">
                {text(
                  lang,
                  'چیست، چه زمانی استفاده شود، مسیر UI، screenshot، مراحل، خروجی‌ها، خطاها و نکات امنیتی.',
                  'What it is, when to use it, UI path, screenshot, steps, outputs, errors, and security notes.',
                )}
              </p>
            </div>
            <div className="rounded-2xl border border-slate-800 bg-slate-900/70 p-5">
              <h3 className="font-bold text-white">{text(lang, 'قانون نگهداری', 'Maintenance rule')}</h3>
              <p className="mt-2 text-sm leading-6 text-slate-400">
                {text(
                  lang,
                  'هر feature جدید باید documentation، screenshot و release notes خودش را همراه patch داشته باشد.',
                  'Every new feature must ship with documentation, screenshots, and release notes updates.',
                )}
              </p>
            </div>
            <div className="rounded-2xl border border-slate-800 bg-slate-900/70 p-5">
              <h3 className="font-bold text-white">{text(lang, 'محدوده اجرای مجاز', 'Authorized execution')}</h3>
              <p className="mt-2 text-sm leading-6 text-slate-400">
                {text(
                  lang,
                  'مستندات باید scope، policy، approval، budget، audit و stop conditions را برای تست‌های فعال توضیح دهد.',
                  'Docs must explain scope, policy, approval, budget, audit, and stop conditions for active testing.',
                )}
              </p>
            </div>
          </section>
        </section>
      </main>
    </div>
  );
}
