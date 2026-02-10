# 🎯 Professional Bug Bounty Hunting Platform

> A scalable, automated, and intelligent reconnaissance framework designed for security researchers and red teamers. Built with performance, modularity, and continuous monitoring in mind.

## 💡 Core Philosophy

Unlike traditional, fire-and-forget scanner scripts, this platform is built as a **Continuous Hunting Machine**.

1.  **No-Waste Strategy:** We store everything. Even currently "dead" subdomains are monitored for future activation (Fresh Asset detection).
2.  **Smart Monitoring:** Distinguishes between existing assets and newly discovered ones.
3.  **Deep Diffing:** Detects not just new subdomains, but changes in existing ones (e.g., Title change, Status Code flip, Tech Stack update).
4.  **Scalability:** Engineered to handle massive targets with hundreds of thousands of assets using batch processing and buffered queues.

## 🏗️ Technical Architecture

The system is built on a modern, containerized microservice-like architecture:

### 🧠 Backend (The Engine)
* **Core:** Golang (Fiber Framework) for high-performance APIs and concurrent workers.
* **Persistence:** PostgreSQL with GORM for structured data storage and JSONB for flexible raw data.
* **Job Queue:** Redis for managing long-running background reconnaissance tasks asynchronously.
* **Notification System:** Buffered Go Channels with rate-limiting to prevent data loss and API blocking (Telegram).
* **Security:** JWT Authentication & Role-Based Access Control (RBAC).
* **Infrastructure:** Fully Dockerized environment with multi-stage builds.

### 💻 Frontend (The Dashboard)
* **Framework:** React.js + Vite (Fast & Modern).
* **Styling:** Tailwind CSS for a professional dark-mode UI.
* **State Management:** TanStack Query (React Query) for efficient API caching and real-time syncing.
* **Features:**
    * **Dashboard:** Live analytics and charts (Recharts).
    * **Monitoring Server:** (Admin Only) Real-time CPU/RAM usage charts and active process list.
    * **Target Management:** Create, Edit, Delete, **Stop/Resume** scans.
    * **Configurable Scans:** Toggle modules like `Alterx`, `Waymore` or `Crawling` per target.
    * **Asset Explorer:** Advanced data grid with Filtering, Search, **Tabs for Assets vs URLs**.
    * **Intel Filtering:** Specific **JS Filtering** and **Multi-Source Filtering** (Wayback, Gau, Katana, Waymore) with **Sorting** capabilities.
    * **Data Import/Export:** Export targets with all related data (Assets, URLs) and import them back with duplicate handling.
    * **User Management:** Admin-only panel to manage team access.
    * **Account:** Self-service account page (view profile + change password).

---

## 🔐 Access Control (RBAC) & Data Isolation

The platform enforces **Role-Based Access Control** across the API and UI.

### Roles
- **admin**
  - Full access to all targets and operations.
  - **Only role allowed** to manage users (`/api/users/*`).
- **viewer** (default)
  - Can only see and operate on targets **created by the same user**.
  - Can manage their own account via `/api/me` (view profile, change password, delete account).

### Target Ownership
Each `Target` has a `created_by_user_id` owner.
- **admin**: can access all targets
- **viewer**: can access only targets where `created_by_user_id == current_user_id`

> Note: targets created before this feature may have `created_by_user_id = 0`. Those targets are only visible to **admin** until you backfill ownership.

## 🛠️ Arsenal (Toolchain)

The platform integrates industry-standard security tools within its isolated environment:

* **Discovery:** 
  * `subfinder`, `assetfinder` (Always enabled)
  * `cero` (**Optional per target**) - Scrape domain names from SSL certificates
  * `crtsh` (**Optional per target**) - Query crt.sh API for subdomain discovery
  * `puredns` (**Optional per target**) - Subdomain bruteforce (wordlist-based) using trusted resolvers (**only live/resolved** results are stored)
* **Permutation/Mutation:** `alterx` (Optional per target)
* **Validation/Resolution:** `dnsx` (w/ fixed resolvers)
* **Probing:** `httpx` (Rich JSON output, WAF/CDN detection)
* **Edge Tech Detection:** `cdncheck` (Early detection from DNS results: **CDN / WAF / CLOUD**, separate from httpx)
* **Port Scanning:** `nmap` (**Optional per target**, runs in Phase 1 on **non-CDN** DNS-resolved IPs)
* **Crawling & Content Discovery:** `gau`, `waybackurls`, `katana` (Active & Passive), `waymore` (Deep Archival Crawl)
* **Deep Scan:** `amass` (Integrated via Docker)
* **Future Integration:** `ffuf`, `nuclei` (Ready in Dockerfile)

---

## 📊 Development Status: Phase 4 Complete

We are following a multi-phase development roadmap.

### ✅ Phase 1: Deep Recon & Discovery Engine (COMPLETED)
**Goal:** Build the core infrastructure and the initial discovery pipeline.
* [x] **Smart Recon Pipeline:** Implemented a full chain (Passive -> Mutation -> Validation).
* [x] **History Injection:** Re-scans previously dead assets to detect resurrections.
* [x] **Smart Storage Logic:** "Upsert" logic to track live/dead status.
* [x] **Enhanced Discovery Tools:** Added `cero` (SSL certificate scraping), `crtsh` (Certificate Transparency API), and `puredns` (bruteforce) as optional tools.
* [x] **Source Tracking:** Each subdomain tracks which tools discovered it (subfinder, assetfinder, cero, crtsh, alterx, puredns).
* [x] **Fresh Asset Logic:** Fixed notification spam - only truly new live subdomains trigger alerts.

### ✅ Phase 2: Probing & Fingerprinting (COMPLETED)
**Goal:** Extract detailed technical intelligence from live assets.
* [x] Integrated `httpx` with rich JSON output parsing.
* [x] **Rich Data Model:** Storing Web Servers, Technologies, IPs, CNAMEs.
* [x] **Batch Processing:** Dynamic batching to handle massive datasets.
* [x] **Diff Engine:** Detects and logs changes in *any* field.

### ✅ Phase 2.5: Automation & Continuous Monitoring (COMPLETED)
**Goal:** Turn the scanner into a 24/7 autonomous monitoring system.
* [x] **Notification System:** Zero-loss Telegram alerting.
* [x] **Scheduler:** Automated periodic scanning.
* [x] **Control:** Full Stop/Resume/Pause capabilities with Kill Switch.

### ✅ Phase 3: Frontend Dashboard & Security (COMPLETED)
**Goal:** A professional GUI to manage targets and secure the platform.
* [x] **Authentication:** Secure Login/Logout with JWT.
* [x] **User Management:** Admin interface.
* [x] **Target Management:** Full CRUD + Configurable Modules.
* [x] **Analytics:** Graphical dashboard with Stat Cards and Charts.

### ✅ Phase 4: Deep Crawling & Content Discovery (COMPLETED)
**Goal:** Harvest URLs, endpoints, and JS files from live assets.
* [x] **Hybrid Crawling:** Integrated `gau`, `waybackurls` (Passive), `katana` (Active) and **Waymore** (Deep History).
* [x] **Smart Deduplication:** Redis Sets used to prevent duplicate URL storage and minimize DB load.
* [x] **Frontend Integration:** Dedicated "Crawled URLs" tab with **JS File Filter**, **Source Filtering**, and **Column Sorting**.
* [x] **Alerting:** Real-time Telegram notifications for fresh URLs.

### ✅ Phase 4.5: Data Import/Export & Module Sorting (COMPLETED)
**Goal:** Enable data portability and ensure proper scan module ordering.
* [x] **Export System:** Full export of targets with all related data (Assets, URLs) in standardized JSON format.
* [x] **Import System:** Complete import with duplicate handling and data validation.
* [x] **Selective Export:** Choose specific targets or export all at once.
* [x] **Module Sorting:** Automatic sorting of scan modules (DISCOVERY → PROBING → CRAWLING) to ensure correct execution order.
* [x] **Data Integrity:** Comprehensive export includes all subdomains, URLs, and metadata for complete data portability.

### ✅ Phase 4.6: CDN/WAF/CLOUD Detection & Infrastructure Setup (COMPLETED)
**Goal:** Enhanced edge technology detection and production-ready deployment infrastructure.
* [x] **Early Edge Tech Detection:** Integrated `cdncheck` for **CDN/WAF/CLOUD** detection from DNS results (before httpx probing).
* [x] **DNS Server:** Self-hosted BIND9 DNS server for complete domain control.
* [x] **SSL/TLS:** Automated Let's Encrypt certificate management with auto-renewal.
* [x] **Reverse Proxy:** Nginx with HTTPS enforcement and security headers.
* [x] **Domain Management:** Dynamic domain configuration via environment variables.

### ✅ Phase 4.7: Enhanced Discovery & Source Tracking (COMPLETED)
**Goal:** Expand discovery capabilities and track subdomain sources.
* [x] **New Discovery Tools:** Added `cero` (SSL certificate scraping), `crtsh` (Certificate Transparency API), and `puredns` (wordlist-based bruteforce) as optional discovery tools.
* [x] **Source Tracking:** Each subdomain now tracks which tools discovered it, displayed as color-coded provider tags in the UI.
* [x] **Smart Source Merging:** When a subdomain is found by multiple tools, all sources are tracked and merged automatically.
* [x] **Fresh Asset Fix:** Improved notification logic to prevent spam - only truly new live subdomains trigger fresh asset alerts.

### ✅ Phase 4.8: Data Export Enhancements (COMPLETED)
**Goal:** Provide flexible data extraction for external tools.
* [x] **Export Assets:** Download filtered subdomains list as `.txt` (Live, Dead, No CDN, etc.) with smart filenames.
* [x] **Export URLs:** Download filtered URL lists as `.txt` (JS only, Specific Sources, etc.).
* [x] **Export IPs:** Download unique IPs of a target as `.txt` for port scanning.

### ✅ Phase 4.9: VirusTotal Integration (COMPLETED)
**Goal:** Enhance intelligence gathering by integrating VirusTotal's API.
* [x] **New Source:** Added VirusTotal as a URL discovery source.
* [x] **Smart Filtering:** Only queries live subdomains to minimize API usage and noise.
* [x] **Source Merging:** Automatically merges VirusTotal findings with other sources (e.g., `gau, virustotal`) for the same URL.
* [x] **Frontend Integration:** Added "VIRUSTOTAL" tag and filter in the URLs tab.

### ✅ Phase 5: System Monitoring & V2.1 Upgrade (COMPLETED)
**Goal:** Provide real-time server health monitoring, enhanced stability, and admin visibility.
* [x] **Monitoring Server:** Added a dedicated, admin-only dashboard section for real-time system metrics.
* [x] **Live Charts:** Interactive Area Charts displaying real-time CPU and RAM usage.
* [x] **Process Tracking:** Detailed table showing all active background scanning processes (PID, Command, Duration).
* [x] **System Logs:** Live terminal-like log viewer (WebSocket) to stream Docker logs directly in the dashboard.
* [x] **V2.1 Architecture:** Upgraded core engine to **Hunt Engine v2.1** with optimized concurrency, panic recovery, and auto-healing locks.
* [x] **Kill Switch V2:** Enhanced process management to reliably track and terminate complex tool chains (pipes, sub-processes).
* [x] **UI Integration:** Export buttons integrated directly into the Asset and URL tables with matching styles.

---

## 🚀 Quick Start

### Prerequisites
* Docker & Docker Compose
* Go 1.21+ (for local development)
* Node.js 18+ (for frontend development)

### Installation

1. **Clone the repository:**
```bash
git clone https://github.com/omidxplimbo/hunt-engine.git
cd hunt-engine
```

2. **Configure environment variables:**
```bash
cp .env.example .env
# Edit .env with your configuration (Database, Redis, Telegram Bot Token, etc.)
```

3. **Start the platform:**
```bash
docker compose up -d
```

4. **Access the dashboard:**
* Frontend: `http://localhost:4000` or `http://localhost:80`
* Backend API: `http://localhost:4000/api` or `http://localhost:80/api`
* Default credentials: `admin` / `admin123` (⚠️ Change in production!)

**Note:** In production, IP access is restricted to the server's own IP. External users must access via domain name.

## 🔒 IP-Based Access Control

The platform implements strict IP-based access control for security:

### Access Rules

1. **Server IP Access:**
   - If you access from the server's own IP (`SERVER_IP`), you can use either:
     - IP address: `http://YOUR_SERVER_IP:4000/` or `http://YOUR_SERVER_IP/`
     - Domain name: `https://yourdomain.com`

2. **External IP Access:**
   - If you access from any other IP address:
     - ❌ IP address access is **blocked** (403 Forbidden)
     - ✅ Domain name access is **allowed** (HTTPS only)

### Configuration

Set `SERVER_IP` in your `.env` file:
```env
SERVER_IP=109.248.160.151  # Your server's public IP
```

### Security Benefits

* **Prevents unauthorized IP access:** Only server administrators can access via IP
* **Forces domain usage:** External users must use the domain name (better for SSL/security)
* **Reduces attack surface:** IP-based attacks are blocked for non-server IPs

### Error Messages

When accessing from an external IP via IP address, you'll see:
```
شما دسترسی ندارید. لطفاً از طریق دامنه وارد شوید.
Access Denied. Please use the domain name to access the site.
```

## 🌐 Production Deployment with Domain & SSL

For production deployment with your own domain and SSL certificate:

### Quick Setup

1. **Configure domain:**
```bash
cp .env.example .env
nano .env
```

Add your domain configuration:
```env
DOMAIN_NAME=yourdomain.com
SSL_EMAIL=admin@yourdomain.com
SERVER_IP=YOUR_SERVER_PUBLIC_IP  # Required for IP-based access control
```

**Important:** `SERVER_IP` is required for IP-based access control. Only requests from this IP can access the site via IP address. All other IPs must use the domain name.

2. **Follow the complete setup guide:**
```bash
cat SETUP-GUIDE.md
```

Or use the quick reference:
```bash
cat QUICK-STEPS.txt
```

### Features

* **Self-Hosted DNS:** BIND9 DNS server for complete domain control
* **SSL/TLS:** Automated Let's Encrypt certificates with auto-renewal
* **HTTPS Only:** Automatic HTTP to HTTPS redirect for domain access
* **IP Access Control:** 
  * Server IP can access via IP address (HTTP/HTTPS)
  * External IPs are blocked from IP access (must use domain)
  * Domain access is always allowed (HTTPS)
* **Security:** IP-based access control, security headers, rate limiting
* **Dynamic Configuration:** Easy domain changes via environment variables
* **Port Configuration:** Access via port 80, 443, or custom port (e.g., 4000)

### Documentation

* **SETUP-GUIDE.md** - Complete step-by-step setup guide
* **DNS-SETUP.md** - DNS server configuration details
* **DOMAIN-SETUP.md** - Domain and SSL setup (alternative method)
* **QUICK-STEPS.txt** - Quick reference checklist

## 📖 Usage Guide

### Target Management

#### Creating a Target
1. Navigate to **Targets** page
2. Click **New Target**
3. Fill in:
   * **Operation Codename:** A friendly name for your target
   * **Target Root:** The root domain (e.g., `example.com`)
   * **Recon Frequency:** Minutes between scans (default: 720 = 12 hours)
   * **Modules:** Select scan phases (DISCOVERY, PROBING, CRAWLING)
   * **Options:** 
     * Toggle **Alterx** (permutation)
     * Toggle **Waymore** (deep crawl)
     * Toggle **Portscan (nmap)** (optional)
     * Toggle **CERO** (SSL certificate scraping) - Optional
     * Toggle **CRT.SH** (Certificate Transparency API) - Optional
     * Toggle **PUREDNS** (subdomain bruteforce) - Optional
       * Select one or more wordlists for this target (multi-select)
       * Only **live/resolved** subdomains are stored and tagged with provider `puredns`

### 📚 Wordlists (Puredns)

Puredns uses wordlists for bruteforcing. You can use:
- **Built-in wordlists** inside container: `/wordlists/*`
- **Custom wordlists** mounted from repo: `./custom_wordlists → /wordlists/custom`

See: **`WORDLISTS-GUIDE.md`** for the full guide (where to put files, format, and examples).

### 🔑 Subfinder Provider API Keys (Per-User)

Subfinder supports many passive sources that require API keys (e.g., `shodan`, `securitytrails`, `chaos`, etc).  
This platform lets **each user** manage their own subfinder provider keys **from the UI**, and they are applied **dynamically at runtime**.

- Keys are stored **per-user** (isolated by account).  
- Keys are **not printed** in logs.  
- When subfinder runs, the engine generates a temporary `provider-config.yaml` and passes it via `subfinder -pc ...` **only for targets owned by that user**.

#### Configure in UI
- Go to **Account** page
- In **Subfinder Provider API Keys**, add providers (example: `shodan`) and paste API key
- Click **Save**

> Note: UI currently supports **one API key string per provider** (it maps to `provider: ["API_KEY"]` in subfinder).  
> Some providers support more complex entries; we can extend the UI later if needed.

#### Exporting Targets
1. Click **Export** button on Targets page
2. Select targets to export (or leave empty to export all)
3. Download includes:
   * Target configuration
   * All discovered subdomains (Assets)
   * All crawled URLs
   * Complete metadata

#### Importing Targets
1. Click **Import** button on Targets page
2. Select exported JSON file
3. Choose **Skip existing targets** to avoid duplicates
4. Import will restore all data including Assets and URLs

### Asset Explorer

#### Filtering Assets
* **Live Only:** Show only active subdomains
* **New Assets:** Recently discovered subdomains
* **DNS Only:** Subdomains without HTTP response
* **No CDN:** Filter assets without CDN protection (both `httpx` CDN + `cdncheck` CDN)
* **CDN:** Show only assets with CDN detected (httpx or cdncheck)
* **WAF:** Show only assets with WAF detected (cdncheck)
* **CLOUD:** Show only assets with CLOUD provider detected (cdncheck)
* **Search:** Filter by domain name
* **Provider Filter (Dropdown):** Filter assets by the discovery provider (subfinder/assetfinder/crtsh/cero/alterx/puredns)
* **Status Filter (Dropdown):** Filter assets by HTTP status code (e.g. `200` or `2xx/3xx/4xx/5xx`)
* **CDN/WAF/CLOUD Badges:** Visual indicators per asset (separate fields)
* **Ports Column:** If Portscan is enabled, open ports (from `nmap`) are shown per asset.
* **Providers Column:** Shows which tools discovered each subdomain with color-coded tags:
  * `subfinder` (Blue)
  * `assetfinder` (Green)
  * `cero` (Purple)
  * `crtsh` (Orange)
  * `alterx` (Yellow)
  * `puredns` (Tagged when discovered via bruteforce + verified live)
  * Multiple tags indicate the subdomain was found by multiple tools

#### Port Scanning (Optional / Phase 1)
When enabled per target (`use_portscan=true`), the engine runs `nmap` in **PHASE 1 (DISCOVERY)** **after** `cdncheck` and scans **only**:
- assets that are **live** (resolved by `dnsx`, have `dnsx_ip`)
- IPs that are **not behind CDN** (based on `cdn_name` + `cdncheck`)

Results are stored in `assets.open_ports` as JSON:
```json
{"1.2.3.4":[80,443]}
```

#### URL Management
* Switch to **URLs** tab to view crawled endpoints
* **JS Files Filter:** Show only JavaScript files
* **Source Filter:** Filter by discovery source (Wayback, GAU, Katana, Waymore)
* **Sorting:** Sort by value, source, or creation date

### Module Execution Order

The platform automatically ensures scan modules execute in the correct order:
1. **DISCOVERY** - Passive subdomain discovery
2. **PROBING** - HTTP probing and fingerprinting
3. **CRAWLING** - URL and endpoint discovery

This order is enforced automatically, even if modules are specified in a different order.

### Crash-Safe Resume (Persistent Checkpoints)

Each target scan is **checkpointed** into the database so you can safely **Pause** and later **Resume**, or recover from server/container crashes **without re-doing completed steps**.

**How it works**
- The worker writes a persistent scan state record (`TargetScanState`) for each target.
- After each major step, the worker marks that step as **completed** and stores minimal progress metadata (e.g. probing batch offset).
- On resume, the worker **skips completed steps** and continues from the next step.

**What is checkpointed**
- **DISCOVERY**: PASSIVE ENUM, ALTERX, PUREDNS, MERGE, DNSX, SAVE, CDNCHECK, NMAP
- **PROBING**: HTTPX batch processing uses a persisted offset so it continues where it left off
- **CRAWLING**: each tool result is saved per-step (Wayback/GAU/Waymore/Katana) so resume does not duplicate work

**Persistence across container restarts**
- The worker workspace (`/tmp/hunt-engine`) is persisted via Docker volume (`hunt_workspaces`) so crash/restart does not wipe intermediate checkpoint artifacts.

## 🔌 API Endpoints

### Authentication
* `POST /api/auth/login` - Login and receive JWT token

### Account (Self-Service)
* `GET /api/me` - Get current user profile
* `PATCH /api/me` - Update current user profile (e.g., username)
* `POST /api/me/change-password` - Change password (requires current password)
* `DELETE /api/me` - Delete own account (requires current password)
* `GET /api/me/subfinder/providers` - List current user's subfinder provider configs
* `PUT /api/me/subfinder/providers` - Replace current user's provider list (used by UI Save)
* `DELETE /api/me/subfinder/providers/:provider` - Remove a single provider for current user

### Targets
* `GET /api/targets` - List all targets (paginated)
* `POST /api/targets` - Create new target
* `GET /api/targets/:id` - Get target details
* `PATCH /api/targets/:id` - Update target
* `DELETE /api/targets/:id` - Delete target
* `POST /api/targets/:id/discovery` - Start scan (**smart**; resumes from checkpoint if possible)
* `POST /api/targets/:id/resume` - Resume scan from last checkpoint (UI uses this)
* `POST /api/targets/:id/stop` - Stop running scan
* `GET /api/targets/:id/scan-state` - Get persistent checkpoint/progress info for this target

### Export/Import
* `GET /api/targets/export` - Export all targets
* `POST /api/targets/export` - Export selected targets (body: `{target_ids: [1,2,3]}`)
* `POST /api/targets/import` - Import targets from JSON

### Assets & URLs
* `GET /api/targets/:id/assets` - Get target assets (with filters)
* `GET /api/targets/:id/urls` - Get target URLs (with filters)
* `GET /api/targets/:id/ips` - Export unique target IPs as TXT (**non-CDN only**)
* `GET /api/wordlists` - List available wordlists from `/wordlists` and `/wordlists/custom` (for Puredns UI)

### Dashboard
* `GET /api/dashboard/stats` - Dashboard statistics (**scoped to current user** unless admin)

### Users (Admin Only)
* `GET /api/users` - List users
* `POST /api/users` - Create user
* `PATCH /api/users/:id` - Update user
* `DELETE /api/users/:id` - Delete user

#### Assets Filters (Query Params)
You can combine these query params on `GET /api/targets/:id/assets`:

* `no_cdn=true` - only assets without CDN (httpx + cdncheck)
* `has_cdn=true` - only assets with CDN detected (httpx or cdncheck)
* `has_waf=true` - only assets with WAF detected (cdncheck)
* `has_cloud=true` - only assets with CLOUD detected (cdncheck)
* `status_code=200` - only assets with exact HTTP status code
* `status_code=2xx` - only assets with status in a class range (supports `2xx/3xx/4xx/5xx`)

## 📦 Export/Import Format

### Export Structure
```json
{
  "version": "1.0",
  "export_date": "2025-12-14T12:00:00Z",
  "targets": [
    {
      "name": "Example Target",
      "root_domain": "example.com",
      "description": "Target description",
      "in_scope": true,
      "frequency": 720,
      "modules": ["DISCOVERY", "PROBING", "CRAWLING"],
      "use_alterx": true,
      "use_waymore": false,
      "use_portscan": false,
      "use_cero": false,
      "use_crtsh": false,
      "use_puredns": false,
      "puredns_wordlists": [],
      "assets": [...],
      "urls": [...]
    }
  ]
}
```

### Import Options
* **Skip Existing:** If enabled, targets with matching `root_domain` are skipped
* **Duplicate Handling:** Assets and URLs are automatically deduplicated during import

## 🏗️ Infrastructure Components

### DNS Server (BIND9)
* Self-hosted DNS server for complete domain control
* Automatic zone file generation
* Name Server configuration (ns1/ns2.yourdomain.com)
* Wildcard subdomain support

### Reverse Proxy (Nginx)
* SSL/TLS termination with Let's Encrypt
* HTTP to HTTPS redirect
* Security headers (HSTS, XSS Protection, etc.)
* Rate limiting
* **IP-based access control:** 
  * Access via IP address is **only allowed from the server's own IP**
  * External IP access attempts are blocked with 403 error
  * Domain access is always allowed (HTTPS)
  * Configure `SERVER_IP` in `.env` to set the allowed server IP

### SSL Management
* Automated certificate generation with Let's Encrypt
* Auto-renewal via cron job
* Certificate monitoring and alerts

## 🔜 Next Steps (Phase 5)

We are now transitioning to **Vulnerability Scanning**.

**Phase 5: Vulnerability Scanning (Backend)**
* Integrate **Nuclei** for template-based scanning.
* Implement **Smart Filtering** (e.g., run WordPress templates only on WordPress sites).
* Immediate Telegram alerts for **Critical/High** vulnerabilities.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📝 License

This project is licensed under the MIT License.

## ⚠️ Disclaimer

This tool is for **authorized security testing and research purposes only**. Users are responsible for ensuring they have proper authorization before scanning any targets.