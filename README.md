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
* **Job Queue:** Redis-backed per-user scan queues for long-running reconnaissance tasks, with configurable concurrent scan slots per user.
* **Notification System:** Buffered Go Channels with rate-limiting to prevent data loss and API blocking (Telegram). Runtime secrets are loaded from environment variables.
* **Security:** JWT Authentication & Role-Based Access Control (RBAC).
* **Infrastructure:** Fully Dockerized environment with multi-stage builds.

### 💻 Frontend (The Dashboard)
* **Framework:** React.js + Vite (Fast & Modern).
* **Styling:** Tailwind CSS for a professional dark-mode UI.
* **State Management:** TanStack Query (React Query) for efficient API caching and real-time syncing.
* **Features:**
    * **Dashboard:**
        * Live analytics and charts (Recharts).
        * **Active Scan Queue:** Real-time view of queued targets. Each user can **Reorder**, **Remove**, or **Clear** only their own scan queue.
    * **Monitoring Server:** (Admin Only) Real-time CPU/RAM usage charts and active process list.
    * **Target Management:** Create, Edit, Delete, **Stop/Resume** scans, with target owner visibility for admins.
    * **Configurable Scans:** Toggle modules like `Alterx`, `Waymore` or `Crawling` per target.
    * **Asset Explorer:** Advanced data grid with Filtering, Search, **Tabs for Assets, URLs, and Findings**.
    * **Intel Filtering:** Specific **JS Filtering**, **Multi-Source Filtering** (Wayback, Gau, Katana, Waymore), Findings filtering, and sorting capabilities.
    * **Data Import/Export:** Export targets with all related data (Assets, URLs) and import them back with duplicate handling.
    * **User Management:** Admin-only panel to manage team access, roles, status, and per-user scan slot limits.
    * **Account:** Self-service account page (view profile, change password, subfinder provider keys, and personal scan queue controls).

---

## 🔐 Access Control (RBAC) & Data Isolation

The platform enforces **Role-Based Access Control** across the API and UI.

### Roles
- **admin**
  - Full visibility across all targets, users, and system-level data.
  - **Only role allowed** to manage users (`/api/users/*`).
  - Can create and manage their own targets and personal queue.
  - Admin-owned scans are not limited by the per-user scan slot cap, but the global engine capacity still applies.
- **viewer** (default)
  - Can only see and operate on targets **created by the same user**.
  - Can manage their own account via `/api/me` (view profile, change password, delete account).
  - Can view and reorder only their own scan queue.

### Target Ownership
Each `Target` has a `created_by_user_id` owner.
- **admin**: can access all targets and sees the target owner in the Targets table.
- **viewer**: can access only targets where `created_by_user_id == current_user_id`.

Target uniqueness is tenant-scoped:
- The same `root_domain` can exist under different users.
- The same user cannot create the same `root_domain` twice.
- Assets are unique per target (`target_id + value`) instead of globally unique.

> Note: targets created before this feature may have `created_by_user_id = 0`. Those targets are only visible to **admin** until you backfill ownership.

### Per-User Queues & Scan Slots
The scanner uses Redis-backed queues isolated per account:

```text
discovery_tasks:user:<user_id>
```

Each user has a configurable `max_concurrent_scans` value managed by admins from the user management UI.

Example:
- If a viewer has `max_concurrent_scans = 3` and starts 5 scans, 3 scans can run immediately and 2 remain queued.
- The viewer can reorder, remove, or clear only their own queued jobs from the Account page.
- Admins can edit each user's slot limit from the Add/Edit User modal.
- Admins still have global target visibility, but queue controls in the Account page operate on the current admin user's own queue.

## 🛠️ Arsenal (Toolchain)

The platform integrates industry-standard security tools within its isolated environment:

* **Discovery:** 
  * `subfinder`, `assetfinder` (Always enabled)
  * `cero` (**Optional per target**) - Scrape domain names from SSL certificates
  * `crtsh` (**Optional per target**) - Query crt.sh API for subdomain discovery
  * `puredns` (**Optional per target**) - Subdomain bruteforce (wordlist-based) using trusted resolvers (**only live/resolved** results are stored). Puredns is an optional Discovery step and does not gate Probing.
* `amass` (**Optional per target**) - OWASP Amass passive enumeration with `AMASS_TIMEOUT_SECONDS` support.
* **Permutation/Mutation:** `alterx` (Optional per target, file-based output with streaming post-processing for large targets)
* **Validation/Resolution:** `dnsx` (streaming batch validation with fixed resolvers)
* **Probing:** `httpx` (Rich JSON output, WAF/CDN detection)
* **Edge Tech Detection:** `cdncheck` (Early detection from DNS results: **CDN / WAF / CLOUD**, separate from httpx)
* **Port Scanning:** `nmap` (**Optional per target**, runs in Phase 1 on **non-CDN** DNS-resolved IPs)
* **Crawling & Content Discovery:** `gau`, `waybackurls`, `katana` (Active & Passive), `waymore` (Deep Archival Crawl)
* **Passive Discovery:** `amass` (**Optional per target**) - OWASP Amass passive enumeration with timeout support
* **Security Scanning:** `nuclei` (Optional per target, profile-aware, custom template support)
* **Future Integration:** `ffuf`

---

## 📊 Development Status: Phase 4 Complete

We are following a multi-phase development roadmap.

### ✅ Phase 1: Deep Recon & Discovery Engine (COMPLETED)
**Goal:** Build the core infrastructure and the initial discovery pipeline.
* [x] **Smart Recon Pipeline:** Implemented a full chain (Passive -> Mutation -> Validation).
* [x] **History Injection:** Re-scans previously dead assets to detect resurrections.
* [x] **Smart Storage Logic:** "Upsert" logic to track live/dead status.
* [x] **Enhanced Discovery Tools:** Added `cero` (SSL certificate scraping), `crtsh` (Certificate Transparency API), and `puredns` (bruteforce) as optional tools.
* [x] **Source Tracking:** Each subdomain tracks which tools discovered it (subfinder, assetfinder, cero, crtsh, alterx, puredns, amass).
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

### ✅ Phase 5.1: SaaS Multi-Tenant Queue Isolation (COMPLETED)
**Goal:** Prepare the platform for multi-user commercial deployment with isolated workspaces and controlled scan capacity.
* [x] **Per-User Queues:** Moved scan scheduling from one shared Redis list to user-specific queues (`discovery_tasks:user:<id>`).
* [x] **Scan Slot Limits:** Added `max_concurrent_scans` to users so admins can define how many scans each viewer can run concurrently.
* [x] **Self-Service Queue Control:** Users can reorder, remove, or clear their own queued scan jobs from the Account page.
* [x] **Admin User Controls:** Admins can set and edit scan slot limits when creating or updating users.
* [x] **Target Owner Visibility:** Admin target views show which user owns each target.
* [x] **Tenant-Scoped Uniqueness:** Targets are unique per user, and assets are unique per target, enabling different users to scan the same root domain independently.
* [x] **Puredns/Probing Fix:** Puredns remains an optional discovery enhancer and no longer affects whether Probing can run.

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

### 👥 User Management & Scan Slots
Admins manage users from **System Config → Users**.

When creating or editing a user, admins can configure:
- **Username**
- **Password** (on create, or when changing credentials)
- **Role** (`admin` or `viewer`)
- **Account status** (`active` / inactive)
- **Scan Queue Slots** (`max_concurrent_scans`)

`max_concurrent_scans` controls how many scans a non-admin user can run at the same time. Extra scan requests remain in that user's Redis queue until capacity is available.

### 📋 Personal Scan Queue
Each user can manage their own scan queue from the **Account** page.

Available actions:
- Move a queued scan to the top of the user's queue
- Move a queued scan to the bottom of the user's queue
- Remove a queued scan
- Clear the user's queued scans

Queue operations are tenant-scoped. A viewer cannot see or mutate another user's queue. Admins can see all targets globally, but Account queue controls apply to the currently logged-in admin user's own queue.

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
* **Provider Filter (Dropdown):** Filter assets by the discovery provider (subfinder/assetfinder/crtsh/cero/alterx/puredns/amass)
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

This order is enforced automatically, even if modules are specified in a different order. Optional Discovery tools such as `puredns` enrich the Discovery phase only; they do not control whether `PROBING` or `CRAWLING` can run.

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
* `GET /api/queue` - List the current user's queued scan jobs
* `DELETE /api/queue` - Clear the current user's queued scan jobs
* `DELETE /api/queue/:index` - Remove a queued scan job owned by the current user
* `POST /api/queue/:index/move-top` - Move a queued scan to the top of the current user's queue
* `POST /api/queue/:index/move-bottom` - Move a queued scan to the bottom of the current user's queue

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
* `POST /api/users` - Create user, including role, status, and `max_concurrent_scans`
* `PATCH /api/users/:id` - Update user, including role, status, and `max_concurrent_scans`
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
* **Skip Existing:** If enabled, targets with matching `root_domain` for the same user are skipped
* **Duplicate Handling:** Assets and URLs are automatically deduplicated during import using tenant/target-scoped uniqueness

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

## 🔜 Next Steps (Phase 6)

We are now transitioning to **Vulnerability Scanning**.

**Phase 6: Vulnerability Scanning (Backend)**
* Integrate **Nuclei** for template-based scanning.
* Implement **Smart Filtering** (e.g., run WordPress templates only on WordPress sites).
* Immediate Telegram alerts for **Critical/High** vulnerabilities.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📝 License

This project is licensed under the MIT License.

## ⚠️ Disclaimer

This tool is for **authorized security testing and research purposes only**. Users are responsible for ensuring they have proper authorization before scanning any targets.

<!-- HUNTENGINE_V31_RUNTIME_START -->
## Runtime Stability & Large Target Processing

Hunt Engine v3.1 adds resource-aware execution paths for large reconnaissance targets.

### Large Alterx Output Handling

Alterx can generate millions of permutation candidates for large root domains. The worker now treats this as a streaming workload:

- Alterx writes output to disk instead of returning huge stdout payloads to the Go backend.
- Alterx output is normalized and de-duplicated line-by-line.
- Long Alterx post-processing updates target progress in the UI, sends scan heartbeats, and reacts to Stop requests.
- Discovery candidate merging streams large mutation files into the DNSX input artifact without materializing the full mutation set in memory.
- Full DNSX validation coverage is preserved; candidates are not dropped just because the target is large.
- Dead Alterx-only candidates are not persisted as millions of database assets; only passive candidates, live DNSX/PureDNS results, and existing assets are persisted.

### Streaming DNSX Validation

DNSX validation runs in configurable batches while reading the input file progressively. This prevents the backend from loading multi-million-line DNS candidate files into memory.

Recommended values:

```env
# Smaller VPS
DNSX_BATCH_SIZE=1000
DNSX_THREADS=20

# Medium server
DNSX_BATCH_SIZE=2000
DNSX_THREADS=30

# Larger server
DNSX_BATCH_SIZE=5000
DNSX_THREADS=50
```

`DNSX_BATCH_SIZE` controls how many candidates are sent to DNSX per execution. It does **not** limit coverage.

### Stop Scan Reliability

External tools are started in their own process group. When a Stop request is issued, the backend kills the full process group instead of only the parent process. This helps prevent child processes from tools such as `dnsx`, `httpx`, `katana`, `puredns`, `massdns`, or shell pipelines from surviving after a scan is stopped.

### Scheduler and Queue Safety

- Scheduled scans reset stale checkpoint state before enqueueing a new periodic run.
- Per-user Redis queues use round-robin selection to reduce cross-user starvation.
- Dispatcher failures, invalid job payloads, unknown job types, and panic recovery now fail/pause the target instead of leaving it stuck in `QUEUED` or `SCANNING`.
- Phase chaining checks Redis enqueue errors so a target is not left `QUEUED` without a corresponding worker job.
<!-- HUNTENGINE_V31_RUNTIME_END -->


<!-- HUNTENGINE_V31_PHASE_START -->
### ✅ Phase 5.2: v3.1 Runtime Hardening & Large Target Stability (COMPLETED)
**Goal:** Make long-running scans safer, more observable, and more resilient for large targets and multi-user deployments.

* [x] **Scheduler Reset Safety:** Periodic scans reset stale scan-state/checkpoint metadata before starting a fresh scheduled run.
* [x] **Process Group Kill:** Stop Scan now terminates full external tool process groups, reducing orphaned child processes.
* [x] **Fair Queue Selection:** Per-user scan queues are selected with round-robin fairness instead of always scanning sorted queue keys from the beginning.
* [x] **Dispatcher Hardening:** Invalid payloads, unknown job types, missing handlers, and panics now fail/pause targets instead of leaving them stuck.
* [x] **Safe Phase Chaining:** Redis enqueue failures while scheduling the next phase now fail/pause the scan instead of leaving a target queued without a job.
* [x] **Streaming Alterx Pipeline:** Alterx output is file-based and post-processed line-by-line with visible progress and heartbeat updates.
* [x] **Streaming DNSX Validation:** DNSX reads candidate files progressively in configurable batches to avoid loading massive candidate sets into backend memory.
* [x] **Resource-Aware Large Target Handling:** Full validation coverage is preserved while avoiding persistence of millions of dead Alterx-only candidates.
<!-- HUNTENGINE_V31_PHASE_END -->


<!-- HUNTENGINE_V31_ENV_START -->
## Environment Configuration

Create a local `.env` file from `.env.example` before starting the stack:

```bash
cp .env.example .env
nano .env
```

Required production values:

```env
DB_USER=hunter
DB_PASSWORD=change_me_to_a_strong_database_password
DB_NAME=huntdb
JWT_SECRET=change_me_to_a_32_plus_character_random_secret
DOMAIN_NAME=yourdomain.com
SSL_EMAIL=admin@yourdomain.com
SERVER_IP=YOUR_SERVER_PUBLIC_IP
```

Generate a strong JWT secret with:

```bash
openssl rand -hex 32
```

Optional integrations:

```env
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
VIRUSTOTAL_API_KEY=
```

If `VIRUSTOTAL_API_KEY` is empty, the VirusTotal crawling source is skipped cleanly. Telegram notification variables can be left empty when notifications are not required.

Large-target runtime tuning:

```env
DNSX_BATCH_SIZE=2000
DNSX_THREADS=30
PROBE_BATCH_SIZE=1000
```

For smaller servers, reduce `DNSX_BATCH_SIZE`, `DNSX_THREADS`, and `PROBE_BATCH_SIZE`. For larger servers, increase them carefully while monitoring CPU/RAM usage.
<!-- HUNTENGINE_V31_ENV_END -->


<!-- HUNTENGINE_V31_TROUBLESHOOTING_START -->
## Troubleshooting Large Discovery Runs

### Alterx appears stuck but Active Processes is empty

For very large targets, Alterx may finish as an external process while the Go backend continues normalizing and de-duplicating the output file. During this time the Active Processes table can show `0`, because the work is internal backend post-processing rather than a child process.

Check progress with:

```bash
docker compose logs -f backend
```

And inspect temporary discovery artifacts:

```bash
docker compose exec backend sh -lc '
find /tmp/hunt-engine -type f \( \
  -name "alterx_results.txt" -o \
  -name "alterx_results.txt.raw" -o \
  -name "dnsx_all_found.txt" \
\) -exec ls -lh {} \; -exec wc -l {} \;
'
```

If `alterx_results.txt` is still increasing, the backend is actively post-processing. The UI should show an `ALTERX POST-PROCESSING` progress phase.

### DNSX is slow on large targets

DNSX can be CPU-intensive when validating millions of candidates. Reduce runtime pressure on smaller servers:

```env
DNSX_BATCH_SIZE=1000
DNSX_THREADS=20
```

This does not reduce coverage; it only lowers the amount of work performed per DNSX batch.

### Emergency recovery for a stuck scan

If a scan must be manually interrupted:

```bash
docker compose stop backend
```

Then inspect queued jobs and scanning targets:

```bash
docker compose exec redis redis-cli keys 'discovery_tasks*'

docker compose exec postgres sh -lc '
psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
SELECT id, root_domain, status, current_phase
FROM targets
WHERE status IN ('\''SCANNING'\'','\''QUEUED'\'')
ORDER BY updated_at DESC;
"
'
```

Pause the affected target from the UI when possible, then restart backend:

```bash
docker compose up -d backend
```
<!-- HUNTENGINE_V31_TROUBLESHOOTING_END -->


### Amass Discovery Option

Amass is available as an optional per-target passive Discovery source. When enabled, Hunt Engine runs:

```bash
amass enum -passive -norecursive -noalts -d <domain>
```

Amass output is written to a temporary file and parsed line-by-line so large passive result sets do not need to be held as one stdout buffer in backend memory. Results are source-tagged as `amass` and then flow into the normal Discovery merge and DNSX validation pipeline.

Runtime tuning:

```env
AMASS_TIMEOUT_SECONDS=900
```

Set `AMASS_TIMEOUT_SECONDS=0` to disable the Amass-specific timeout. Stop Scan still uses the worker process-group kill path.
<!-- V3_1_RUNTIME_DOCS_START -->

---

## v3.1.0 Runtime Hardening & Large Target Stability

Version `v3.1.0` focuses on safer large-target reconnaissance, better scan observability, and optional Amass passive enumeration.

### Large Target Discovery Improvements

- **Alterx streaming pipeline**
  - Alterx output is written to files instead of being captured as a huge in-memory stdout buffer.
  - Large Alterx outputs are normalized line-by-line.
  - Alterx post-processing reports visible UI progress, heartbeat updates, and reacts to Stop requests.

- **DNSX streaming validation**
  - DNSX input is processed in configurable batches.
  - The backend no longer loads the full candidate file into memory before validation.
  - Full candidate coverage is preserved while keeping memory usage bounded.

- **PureDNS runtime visibility**
  - PureDNS internal phases are clearer in the UI.
  - Stop-check polling is throttled so large internal loops do not hammer the database with `stop_requested` queries.

### Optional Amass Passive Discovery

Amass can now be enabled per target from the Create/Edit Target modals.

When enabled, Amass runs during **Discovery → Passive Enumeration** and contributes source-tagged subdomains as provider `amass`.

The worker includes compatibility handling for different Amass CLI behavior and supports timeout-based execution.

### Amass Timeout

Use:

```env
AMASS_TIMEOUT_SECONDS=900
```

Behavior:

- `900` means Amass can run for up to 15 minutes.
- `0` disables the Amass-specific timeout.
- Stop Scan still kills the Amass process group.

### Resource Tuning

For small/medium servers:

```env
DNSX_BATCH_SIZE=2000
DNSX_THREADS=30
AMASS_TIMEOUT_SECONDS=900
```

For larger servers:

```env
DNSX_BATCH_SIZE=5000
DNSX_THREADS=50
AMASS_TIMEOUT_SECONDS=900
```

These values do not reduce coverage. They control batch size, runtime pressure, and timeout behavior.

### Worker Stability Improvements

- Scheduled scans reset stale checkpoint metadata before fresh periodic runs.
- Stop Scan kills full external tool process groups, not just parent processes.
- Per-user queues are scheduled with round-robin fairness.
- Dispatcher failures pause/fail targets instead of leaving them stuck in `QUEUED` or `SCANNING`.
- Phase chaining handles Redis enqueue failures safely.
- Runtime secrets are read from environment variables.

### Troubleshooting Large Discovery Runs

If the UI shows an internal phase but Active Processes is empty, the backend may be doing internal file processing rather than running an external binary.

Useful commands:

```bash
docker compose logs -f backend

docker compose exec backend sh -lc '
ps -eo pid,pgid,ppid,etime,pcpu,pmem,comm,args | grep -E "alterx|dnsx|puredns|amass|hunt-engine-api" | grep -v grep
'

docker compose exec backend sh -lc '
find /tmp/hunt-engine -type f \( \
  -name "alterx_results.txt" -o \
  -name "alterx_results.txt.raw" -o \
  -name "dnsx_all_found.txt" -o \
  -name "*amass*" -o \
  -name "*puredns*" \
\) -exec ls -lh {} \; -exec wc -l {} \; 2>/dev/null
'
```

<!-- V3_1_RUNTIME_DOCS_END -->

<!-- V3_1_POST_RELEASE_CLEANUP_START -->

---

## v3.1 Runtime Configuration & Operations

### Required environment variables

Before starting the stack, copy `.env.example` to `.env` and set production-safe values:

```bash
cp .env.example .env
nano .env
```

Minimum required values:

```env
DB_PASSWORD=change_me_to_a_strong_database_password
JWT_SECRET=change_me_to_a_random_64_hex_secret
```

Generate a strong JWT secret with:

```bash
openssl rand -hex 32
```

### Discovery tuning

Large discovery runs can be tuned without reducing coverage:

```env
DNSX_BATCH_SIZE=2000
DNSX_THREADS=30
AMASS_TIMEOUT_SECONDS=900
PROBE_BATCH_SIZE=1000
```

Recommended defaults:

- Small VPS: `DNSX_BATCH_SIZE=1000`, `DNSX_THREADS=20`
- Medium server: `DNSX_BATCH_SIZE=2000`, `DNSX_THREADS=30`
- Larger server: `DNSX_BATCH_SIZE=5000`, `DNSX_THREADS=50`

### Optional Amass passive enumeration

`amass` is available as an optional per-target Discovery provider. Enable **AMASS** in the Create/Edit Target modal to run passive Amass enumeration during Discovery.

Behavior:

- Results are source-tagged as `amass`.
- Provider filtering supports `amass` in the asset table.
- `AMASS_TIMEOUT_SECONDS=900` limits Amass to 15 minutes by default.
- `AMASS_TIMEOUT_SECONDS=0` disables the Amass-specific timeout.
- Stop Scan still kills the Amass process group.

### Large target stability

v3.1 improves large-target handling:

- Alterx writes output to files and normalizes large output line-by-line.
- Alterx post-processing reports visible progress and heartbeat updates.
- DNSX validates candidates using streaming batches instead of loading full candidate files into memory.
- PureDNS and large discovery loops throttle stop checks to avoid excessive database polling.
- External tools are killed by process group on Stop Scan.

### Troubleshooting useful commands

```bash
docker compose logs -f backend

docker compose exec backend sh -lc '
ps -eo pid,pgid,ppid,etime,pcpu,pmem,comm,args | grep -E "alterx|dnsx|puredns|amass|hunt-engine-api" | grep -v grep
'

docker compose exec backend sh -lc '
find /tmp/hunt-engine -type f \(   -name "alterx_results.txt" -o   -name "alterx_results.txt.raw" -o   -name "dnsx_all_found.txt" -o   -name "*amass*" -o   -name "*puredns*" \) -exec ls -lh {} \; -exec wc -l {} \; 2>/dev/null
'
```

<!-- V3_1_POST_RELEASE_CLEANUP_END -->
<!-- V3_2_FINDINGS_DOCS_START -->
---

## v3.2.0 Findings System

Version `v3.2.0` introduces the first production-ready Finding layer on top of Hunt Engine's reconnaissance pipeline. The goal is to move beyond raw asset and URL collection and turn selected signals into structured, triageable, exportable security findings.

### What Findings Add

Findings are structured records that connect a target, optional asset, optional URL, severity, evidence, recommendation, status, and timestamps.

Each finding includes:

```text
severity: info / low / medium / high / critical
status: open / accepted / false_positive / fixed
source_tool: builtin / builtin-url / future engines
first_seen / last_seen
triage_note / triaged_at / triaged_by_user_id
fingerprint for stable deduplication
```

### Built-in Asset Findings

After the Probing phase updates assets, Hunt Engine can generate lightweight builtin findings from asset metadata:

- Possible exposed admin/login interfaces
- Possible directory listings
- HTTP 5xx server errors
- Potentially exposed sensitive services based on risky open ports

These detections are intentionally conservative and are designed to highlight items for manual review rather than claim exploitability.

### Built-in URL Findings

After the Crawling phase completes, Hunt Engine inspects discovered URLs and creates findings for patterns such as:

- Admin/login/dashboard paths
- Exposed configuration or secret-looking paths such as `.env`, `config.php`, `wp-config.php`, `database.yml`
- Version-control paths such as `.git`, `.svn`, `.hg`
- API documentation and schema paths such as `swagger`, `openapi`, `graphql`
- Debug and monitoring paths such as `actuator`, `metrics`, `server-status`, `phpinfo`
- Backup/archive artifacts such as `.bak`, `.old`, `.zip`, `.sql`, `.dump`

### URL Canonicalization and Noise Reduction

URL findings use canonical URLs to reduce duplicate noise caused by volatile query parameters.

Examples of volatile parameters removed from finding fingerprints include:

```text
csrf, nonce, token, session, sid, signature, timestamp, cache, utm_*, fbclid, gclid
```

Meaningful parameter names can still be preserved while values are removed. For example:

```text
/product?id=123
/product?id=456
```

can be treated as:

```text
/product?id
```

Finding evidence stores a cleaner canonical URL plus a shortened raw sample URL:

```text
canonical_url=https://example.com/login
sample_raw_url=https://example.com/login?csrf=...
source=wayback
```

### Canonical URL Storage Dedupe

Newly stored crawled URLs are also deduplicated by canonical hash. This keeps noisy archival variants from creating unnecessary database growth while preserving a representative raw sample URL, occurrence count, last seen timestamp, and merged sources.

> Existing historical URL records are not automatically merged. This avoids destructive migrations and keeps the upgrade safe.

### Findings UI

The target details page includes a dedicated **Findings** tab with:

- Summary cards for total, open, high+, and fixed findings
- Severity, status, and search filters
- Evidence and recommendation display
- Triage status changes
- Optional triage notes through an in-app modal
- CSV and JSON export actions

### Findings API

Key endpoints:

```text
GET   /api/findings
GET   /api/targets/:id/findings
GET   /api/targets/:id/findings/stats
GET   /api/targets/:id/findings/export?format=csv
GET   /api/targets/:id/findings/export?format=json
PATCH /api/findings/:id/status
```

All Findings endpoints enforce the same target ownership rules as the rest of the platform: admins can access all targets; regular users can only access findings for their own targets.

### AI-Ready Direction

The Findings layer is intentionally structured so future AI features can operate as a safe, optional analysis layer. AI should analyze findings, evidence, stats, and scan history asynchronously without blocking the core scan pipeline. If AI is disabled or fails, scans, findings, exports, and triage continue to work normally.
<!-- V3_2_FINDINGS_DOCS_END -->

---
## v3.3.0 Release Summary: Nuclei Security Engine

This release adds the Nuclei Security Engine and the first AI-ready template workflow foundation.

### Added

- Nuclei execution after probing for live HTTP assets.
- Nuclei findings ingestion into Hunt findings.
- Canonical scan profiles: `safe`, `fast`, `balanced`, `cves-light`, `full`, and `custom`.
- Profile-aware custom template placement.
- Custom Nuclei template management API and UI.
- Template validation through the backend before saving.
- Database-backed, per-user custom Nuclei templates.
- Runtime filesystem cache for Nuclei execution.
- AI template draft foundation, disabled by default.
- AI-ready template strategy endpoint for future agent workflows.

### Fixed

- `use_nuclei` and `nuclei_profile` are persisted correctly on target create/update.
- `fast` and `balanced` are no longer normalized back to `safe`.
- Target delete safely removes related findings before assets.
- Custom template delete uses database records instead of fragile filesystem paths.
- Custom templates are isolated per user.

### Safety Defaults

- AI-generated templates are draft-only.
- AI template drafting is disabled by default with `NUCLEI_ALLOW_AI_TEMPLATES=false`.
- No automatic template saving.
- No automatic template execution.
- Human review and validation are required before use.

---
## Nuclei Security Engine

Hunt Engine can run Nuclei after probing when a target has `use_nuclei=true`.

### Target Scan Profiles

Targets can use one of these Nuclei profiles:

- `safe` - conservative checks for medium/high/critical issues.
- `fast` - lightweight exposure and panel checks.
- `balanced` - broader misconfiguration and exposure checks.
- `cves-light` - focused CVE-oriented checks.
- `full` - broad template coverage.
- `custom` - custom-focused execution for advanced workflows.

### Custom Nuclei Templates

Custom templates are stored in PostgreSQL as the source of truth and scoped per user. During scan execution, enabled templates for the target owner are materialized into a runtime filesystem cache and passed to Nuclei with `-t`.

Template placements control which profiles execute a template:

- `root`, `shared`, `safe` - available to every profile.
- `fast`, `exposure` - available to `fast`, `balanced`, `full`, and `custom`.
- `balanced`, `misconfig` - available to `balanced`, `full`, and `custom`.
- `cves`, `cves-light` - available to `cves-light`, `full`, and `custom`.
- `full`, `custom` - available to `full` and `custom`.

### AI-Ready Template Foundation

The AI template draft foundation is disabled by default. When enabled, it returns draft-only templates and strategy recommendations; it never saves or executes templates automatically.

Relevant environment variable:

```env
NUCLEI_ALLOW_AI_TEMPLATES=false
```

Relevant API areas:

```text
GET    /api/nuclei/templates
POST   /api/nuclei/templates
POST   /api/nuclei/templates/validate
DELETE /api/nuclei/templates/:id
GET    /api/nuclei/template-drafts/status
POST   /api/nuclei/template-drafts
GET    /api/nuclei/template-drafts/targets/:id/strategy
```

