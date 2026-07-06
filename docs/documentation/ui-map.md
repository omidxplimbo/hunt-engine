# Hunt Engine UI Map

## Public routes

| Route | Page | Auth | Purpose |
|---|---|---|---|
| `/` | Landing | Public | Public product entry |
| `/login` | Login | Public | Authentication |
| `/documentation` | Documentation | Public | Product documentation |

## Protected routes

| Route | Page | Purpose |
|---|---|---|
| `/dashboard` | Dashboard | Command center overview |
| `/account` | Account | User profile and personal settings |
| `/targets` | TargetsPage | Target management |
| `/targets/:id` | TargetAssets | Target workspace |
| `/operator-learning` | OperatorLearning | Methodology records |
| `/operator-skills` | OperatorSkills | Executable/user-defined skills |

## Admin routes

| Route | Page | Purpose |
|---|---|---|
| `/nuclei-templates` | NucleiTemplates | Nuclei template management |
| `/settings` | Settings | System configuration |

## Main navigation

- Dashboard
- Account
- Targets
- Operator Learning
- Executable Skills
- Nuclei Templates
- System Config
- Disconnect

## Target workspace tabs

- PDF Report
- Assets
- Intel / URLs
- Policy
- Findings
- Analysis
- Export IPs
- Export Assets
- Export URLs

## Analysis subpanels

- AI Analysis
- Recommendations
- Advisory Agents
- Agent Actions
- Bug Tests
- Pattern Registry
- Payload Registry
- Operator Profile
- Attack Surface Chat

## Settings panels

- Users
- Queue Manager
- Concurrency Config
- Wordlists Config
- PureDNS Resolver Config
- LLM Provider Config
- Telegram Config
- VirusTotal Config
- Monitoring Server
- System Logs
- Feature Flags
