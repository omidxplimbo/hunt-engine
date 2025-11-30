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
* **Infrastructure:** Fully Dockerized environment with multi-stage builds.

### 💻 Frontend (The Dashboard)
* **Framework:** React.js + Vite (Fast & Modern).
* **Styling:** Tailwind CSS for a professional dark-mode UI.
* **State Management:** TanStack Query (React Query) for efficient API caching and syncing.
* **Features:** Target management, Asset exploration (Filtering, Searching, IP details).

## 🛠️ Arsenal (Toolchain)

The platform integrates industry-standard Golang-based security tools within its isolated environment:

* **Discovery:** `subfinder`, `assetfinder`
* **Permutation/Mutation:** `alterx`
* **Validation/Resolution:** `puredns` (w/ fixed resolvers), `dnsx`
* **Probing:** `httpx` (Rich JSON output, WAF/CDN detection)
* **Deep Scan:** `amass` (Integrated via Docker)

---

## 📊 Development Status: Phase 3 Complete

We are following a multi-phase development roadmap.

### ✅ Phase 1: Deep Recon & Discovery Engine (COMPLETED)
**Goal:** Build the core infrastructure and the initial discovery pipeline to find hidden and fresh assets.
* [x] Established full Docker/Go/Postgres/Redis infrastructure.
* [x] **Smart Recon Pipeline:** Implemented a full chain (Passive -> Mutation -> Validation).
* [x] **History Injection:** Re-scans previously dead assets to detect resurrections.
* [x] **Smart Storage Logic:** "Upsert" logic to track live/dead status.

### ✅ Phase 2: Probing & Fingerprinting (COMPLETED)
**Goal:** Extract detailed technical intelligence from live assets.
* [x] Integrated `httpx` with rich JSON output parsing.
* [x] **Rich Data Model:** Storing Web Servers, Technologies, IPs, CNAMEs.
* [x] **Batch Processing:** Dynamic batching to handle massive datasets without OOM errors.
* [x] **Diff Engine:** Detects and logs changes in *any* field (Status, Title, Tech, IP).

### ✅ Phase 2.5: Automation & Continuous Monitoring (COMPLETED)
**Goal:** Turn the scanner into a 24/7 autonomous monitoring system.
* [x] **Asset History:** Keeps a detailed audit log of what changed and when.
* [x] **Notification System:** Zero-loss Telegram alerting with buffered queues.
* [x] **Scheduler:** Automated periodic scanning based on per-target frequency.
* [x] **Orchestrator:** Smart chaining that automatically triggers Phase 2 after Phase 1.

### ✅ Phase 3: Frontend Dashboard (COMPLETED)
**Goal:** A professional GUI to manage targets and view results.
* [x] **Target Management:** Create, Edit, Delete targets with config (Frequency, Modules).
* [x] **Asset Explorer:** Advanced data grid with **Filtering** (Live/Dead) and **Search**.
* [x] **Real-time UX:** Responsive UI with live status updates and horizontal scrolling for details.

---

## 🔜 Next Steps (Phase 4 & 5)

We are now transitioning to **Advanced Content Discovery** and **Vulnerability Scanning**.

**Phase 4: Deep Crawling & Content Discovery (Backend)**
* Integrate `gau`, `waybackurls` for passive URL discovery.
* Integrate `katana` for active crawling and JS file discovery.
* Store parsed URLs and JS Secrets in new database tables.

**Phase 5: Vulnerability Scanning (Backend)**
* Integrate **Nuclei** for template-based scanning.
* Implement **Smart Filtering** (e.g., run WordPress templates only on WordPress sites).
* Immediate Telegram alerts for **Critical/High** vulnerabilities.