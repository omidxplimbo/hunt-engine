import { Link } from "react-router-dom";
import {
  ArrowRight,
  Cpu,
  Database,
  Lock,
  Radar,
  ShieldCheck,
  Terminal,
  Zap,
} from "lucide-react";

const capabilities = [
  {
    title: "Attack Surface Intelligence",
    text: "Map assets, URLs, parameters, technologies, and exposure signals from one controlled research workspace.",
    icon: Radar,
  },
  {
    title: "Controlled Pentest Operator",
    text: "Move from reconnaissance to evidence-driven validation with policy-aware execution and auditable decisions.",
    icon: ShieldCheck,
  },
  {
    title: "Target Memory",
    text: "Preserve context, findings, failed hypotheses, confirmed evidence, and operator decisions across every run.",
    icon: Database,
  },
];

const bootLines = [
  "loading operator interface",
  "initializing recon telemetry",
  "mounting target memory",
  "arming controlled runtime",
  "awaiting authorized session",
];

const Landing = () => {
  return (
    <div className="min-h-screen overflow-hidden bg-hack-bg bg-grid-pattern bg-[size:40px_40px] text-white relative">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_20%,rgba(0,255,65,0.16),transparent_34%),linear-gradient(to_bottom,transparent,rgba(0,0,0,0.86))]" />
      <div className="absolute inset-x-0 top-0 h-px bg-hack-primary/70 shadow-[0_0_28px_rgba(0,255,65,0.7)]" />
      <div className="absolute left-10 top-16 hidden select-none font-display text-9xl text-hack-dim/10 md:block">
        01
      </div>
      <div className="absolute bottom-12 right-10 hidden select-none font-display text-9xl text-hack-dim/10 md:block">
        10
      </div>

      <main className="relative z-10 mx-auto flex min-h-screen w-full max-w-7xl flex-col px-5 py-8 md:px-8 lg:px-10">
        <header className="flex items-center justify-between border-b border-hack-primary/20 pb-5">
          <div className="flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-full border border-hack-primary/40 bg-hack-primary/10 shadow-[0_0_22px_rgba(0,255,65,0.18)]">
              <Terminal className="text-hack-primary" size={24} />
            </div>
            <div>
              <p className="font-display text-xl tracking-[0.24em] text-hack-primary">
                MUSTACHE
              </p>
              <p className="font-mono text-[10px] uppercase tracking-[0.35em] text-hack-dim">
                Security Team
              </p>
            </div>
          </div>

          <div className="hidden items-center gap-2 font-mono text-[10px] uppercase tracking-[0.28em] text-hack-dim md:flex">
            <span className="h-2 w-2 rounded-full bg-hack-primary shadow-[0_0_12px_rgba(0,255,65,0.8)]" />
            Public Access Node
          </div>
        </header>

        <section className="grid flex-1 items-center gap-10 py-12 lg:grid-cols-[1.08fr_0.92fr] lg:py-20">
          <div className="animate-in fade-in slide-in-from-bottom-6 duration-700">
            <div className="mb-6 inline-flex items-center gap-2 border border-hack-primary/30 bg-hack-primary/10 px-3 py-2 font-mono text-[10px] uppercase tracking-[0.28em] text-hack-primary">
              <Zap size={13} />
              Offensive Security Research Platform
            </div>

            <h1 className="font-display text-4xl leading-tight tracking-[0.18em] text-hack-primary drop-shadow-[0_0_18px_rgba(0,255,65,0.38)] md:text-6xl lg:text-7xl">
              HUNT THE UNKNOWN.
              <br />
              VALIDATE WITH EVIDENCE.
            </h1>

            <p className="mt-6 max-w-2xl font-mono text-sm leading-7 text-hack-dim md:text-base">
              Mustache Security Team builds controlled, evidence-driven security
              automation for modern attack surface research. Our platform helps
              teams map exposure, preserve target context, validate findings, and
              transform raw reconnaissance into actionable vulnerability
              intelligence.
            </p>

            <div className="mt-8 flex flex-col gap-3 sm:flex-row">
              <Link
                to="/login"
                className="hack-btn group inline-flex items-center justify-center gap-2 px-6 py-4 text-sm"
              >
                INITIALIZE SESSION
                <ArrowRight
                  size={16}
                  className="transition-transform group-hover:translate-x-1"
                />
              </Link>
              <a
                href="mailto:contact@mustache.security?subject=Request%20Access%20to%20Mustache%20Security%20Platform"
                className="hack-btn-ghost inline-flex items-center justify-center gap-2 border border-hack-border px-6 py-4 text-sm hover:border-hack-primary hover:text-hack-primary"
              >
                REQUEST ACCESS
              </a>
            </div>

            <div className="mt-10 grid gap-3 sm:grid-cols-3">
              {[
                ["Controlled", "Policy-aware execution"],
                ["Auditable", "Evidence-first workflow"],
                ["Operator-led", "Human approval gates"],
              ].map(([label, value]) => (
                <div
                  key={label}
                  className="border border-hack-border/70 bg-black/30 p-4 backdrop-blur"
                >
                  <p className="font-display text-lg tracking-[0.18em] text-hack-primary">
                    {label}
                  </p>
                  <p className="mt-1 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                    {value}
                  </p>
                </div>
              ))}
            </div>
          </div>

          <div className="hack-box relative overflow-hidden p-1 animate-in fade-in zoom-in duration-700">
            <div className="absolute inset-0 bg-hack-primary/5 blur-2xl" />
            <div className="relative border border-hack-primary/25 bg-hack-panel/90 p-5 backdrop-blur-xl md:p-6">
              <div className="mb-5 flex items-center justify-between border-b border-hack-border/50 pb-4">
                <div className="flex items-center gap-2 font-mono text-xs uppercase tracking-[0.24em] text-hack-primary">
                  <Cpu size={16} />
                  Operator Boot
                </div>
                <div className="font-mono text-[10px] text-hack-dim">
                  SYS.PUBLIC.GATE
                </div>
              </div>

              <div className="space-y-3 font-mono text-xs">
                {bootLines.map((line, index) => (
                  <div
                    key={line}
                    className="flex items-center justify-between border border-hack-border/50 bg-black/35 px-3 py-3"
                    style={{ animationDelay: `${index * 110}ms` }}
                  >
                    <span className="text-hack-dim">
                      <span className="text-hack-primary">&gt;_</span> {line}
                    </span>
                    <span className="text-hack-primary">OK</span>
                  </div>
                ))}
              </div>

              <div className="mt-6 border border-hack-primary/30 bg-hack-primary/5 p-4">
                <div className="flex items-start gap-3">
                  <Lock className="mt-1 text-hack-primary" size={18} />
                  <div>
                    <p className="font-display text-xl tracking-[0.18em] text-hack-primary">
                      ACCESS CONTROLLED
                    </p>
                    <p className="mt-2 font-mono text-xs leading-6 text-hack-dim">
                      This platform is available to authorized operators,
                      customers, and approved security research teams only.
                    </p>
                  </div>
                </div>
              </div>

              <div className="pointer-events-none absolute inset-x-0 top-0 h-16 animate-pulse bg-gradient-to-b from-hack-primary/10 to-transparent" />
            </div>
          </div>
        </section>

        <section className="grid gap-4 pb-10 md:grid-cols-3">
          {capabilities.map((item) => {
            const Icon = item.icon;
            return (
              <div
                key={item.title}
                className="group border border-hack-border bg-hack-panel/55 p-5 backdrop-blur transition-all duration-300 hover:border-hack-primary/60 hover:bg-hack-primary/5"
              >
                <div className="mb-4 flex h-10 w-10 items-center justify-center border border-hack-primary/30 bg-hack-primary/10 text-hack-primary">
                  <Icon size={19} />
                </div>
                <h2 className="font-display text-lg tracking-[0.16em] text-hack-primary">
                  {item.title}
                </h2>
                <p className="mt-3 font-mono text-xs leading-6 text-hack-dim">
                  {item.text}
                </p>
              </div>
            );
          })}
        </section>
      </main>
    </div>
  );
};

export default Landing;
