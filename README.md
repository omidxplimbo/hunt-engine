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
    * **Target Management:** Create, Edit, Delete, **Stop/Resume** scans.
    * **Configurable Scans:** Toggle modules like `Alterx`, `Waymore` or `Crawling` per target.
    * **Asset Explorer:** Advanced data grid with Filtering, Search, **Tabs for Assets vs URLs**.
    * **Intel Filtering:** Specific **JS Filtering** and **Multi-Source Filtering** (Wayback, Gau, Katana, Waymore) with **Sorting** capabilities.
    * **Data Import/Export:** Export targets with all related data (Assets, URLs) and import them back with duplicate handling.
    * **User Management:** Admin panel to manage team access.

## 🛠️ Arsenal (Toolchain)

The platform integrates industry-standard security tools within its isolated environment:

* **Discovery:** `subfinder`, `assetfinder`
* **Permutation/Mutation:** `alterx` (Optional per target)
* **Validation/Resolution:** `dnsx` (w/ fixed resolvers)
* **Probing:** `httpx` (Rich JSON output, WAF/CDN detection)
* **CDN Detection:** `cdncheck` (Early CDN detection from DNS results, separate from httpx)
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

### ✅ Phase 4.6: CDN Detection & Infrastructure Setup (COMPLETED)
**Goal:** Enhanced CDN detection and production-ready deployment infrastructure.
* [x] **Early CDN Detection:** Integrated `cdncheck` for CDN detection from DNS results before httpx probing.
* [x] **DNS Server:** Self-hosted BIND9 DNS server for complete domain control.
* [x] **SSL/TLS:** Automated Let's Encrypt certificate management with auto-renewal.
* [x] **Reverse Proxy:** Nginx with HTTPS enforcement and security headers.
* [x] **Domain Management:** Dynamic domain configuration via environment variables.

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
* Frontend: `http://localhost:4000`
* Backend API: `http://localhost:8080/api`
* Default credentials: `admin` / `admin123` (⚠️ Change in production!)

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
SERVER_IP=YOUR_SERVER_PUBLIC_IP
```

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
* **HTTPS Only:** Automatic HTTP to HTTPS redirect
* **Security:** IP blocking, security headers, rate limiting
* **Dynamic Configuration:** Easy domain changes via environment variables

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
   * **Options:** Toggle Alterx (permutation) and Waymore (deep crawl)

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
* **No CDN:** Filter assets without CDN protection
* **Search:** Filter by domain name
* **CDN Detection:** Visual indicator for CDN-detected assets (via cdncheck)

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

## 🔌 API Endpoints

### Authentication
* `POST /api/auth/login` - Login and receive JWT token

### Targets
* `GET /api/targets` - List all targets (paginated)
* `POST /api/targets` - Create new target
* `GET /api/targets/:id` - Get target details
* `PATCH /api/targets/:id` - Update target
* `DELETE /api/targets/:id` - Delete target
* `POST /api/targets/:id/discovery` - Start/resume scan
* `POST /api/targets/:id/stop` - Stop running scan

### Export/Import
* `GET /api/targets/export` - Export all targets
* `POST /api/targets/export` - Export selected targets (body: `{target_ids: [1,2,3]}`)
* `POST /api/targets/import` - Import targets from JSON

### Assets & URLs
* `GET /api/targets/:id/assets` - Get target assets (with filters)
* `GET /api/targets/:id/urls` - Get target URLs (with filters)

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
* IP-based access control (domain-only access)

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