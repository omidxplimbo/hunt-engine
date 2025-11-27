# 🎯 Professional Bug Bounty Hunting Platform

> A scalable, automated, and intelligent reconnaissance framework designed for security researchers and red teamers. Built with performance, modularity, and continuous monitoring in mind.



[Image of futuristic cybersecurity dashboard interface]


## 💡 Core Philosophy

Unlike traditional, fire-and-forget scanner scripts, this platform is built as a **Continuous Hunting Machine**.

1.  **No-Waste Strategy:** We store everything. Even currently "dead" subdomains are monitored for future activation (Fresh Asset detection).
2.  **Smart Monitoring:** Distinguishes between existing assets and newly discovered ones.
3.  **Deep Diffing:** Detects not just new subdomains, but changes in existing ones (e.g., Title change, Status Code flip, Tech Stack update).
4.  **Scalability:** Engineered to handle massive targets with hundreds of thousands of assets using batch processing and buffered queues.

## 🏗️ Technical Architecture

The system is built on a modern, containerized microservice-like architecture:

* **Backend Core:** Golang (Fiber Framework) for high-performance APIs and concurrent workers.
* **Persistence:** PostgreSQL with GORM for structured data storage and JSONB for flexible raw data.
* **Job Queue:** Redis for managing long-running background reconnaissance tasks asynchronously.
* **Notification System:** Buffered Go Channels with rate-limiting to prevent data loss and API blocking.
* **Infrastructure:** Fully Dockerized environment with multi-stage builds.

## 🛠️ Arsenal (Toolchain)

The platform integrates industry-standard Golang-based security tools within its isolated environment:

* **Discovery:** `subfinder`, `assetfinder`
* **Permutation/Mutation:** `alterx`
* **Validation/Resolution:** `puredns` (w/ fixed resolvers), `dnsx`
* **Probing:** `httpx` (Rich JSON output)
* **Deep Scan:** `amass` (Integrated via Docker)

---

## 📊 Development Status: Phase 2.5 Complete

We are following a multi-phase development roadmap.

### ✅ Phase 1: Deep Recon & Discovery Engine (COMPLETED)

**Goal:** Build the core infrastructure and the initial discovery pipeline to find hidden and fresh assets.

**Achievements:**
- [x] Established full Docker/Go/Postgres/Redis infrastructure.
- [x] **Smart Recon Pipeline:** Implemented a full chain (Passive -> Mutation -> Validation).
- [x] **History Injection:** Re-scans previously dead assets to detect resurrections.
- [x] **Smart Storage Logic:** Implemented "upsert" logic to track live/dead status.
- [x] **Data Retrieval APIs:** Implemented paginated/filtered APIs.

### ✅ Phase 2: Probing & Fingerprinting (COMPLETED)

**Goal:** Extract detailed technical intelligence from live assets.

**Achievements:**
- [x] Integrated `httpx` with rich JSON output parsing.
- [x] **Rich Data Model:** Storing Web Servers, Technologies, IPs (A records), CNAMEs, and raw JSONB data.
- [x] **Batch Processing:** Implemented dynamic batching (e.g., 500/batch) to handle massive datasets without OOM errors.
- [x] **Input-Based Matching:** Solved CDN/WAF IP masking issues by mapping inputs to database records.

### ✅ Phase 2.5: Automation & Continuous Monitoring (COMPLETED)

**Goal:** Turn the scanner into a 24/7 autonomous monitoring system.

**Achievements:**
- [x] **Diff Engine:** Detects and logs changes in *any* field (Status, Title, Tech, IP) between scans.
- [x] **Asset History:** Keeps a detailed audit log of what changed and when.
- [x] **Notification System:** Zero-loss Telegram alerting with buffered queues (50k capacity) and backpressure handling.
- [x] **Scheduler:** Automated periodic scanning based on per-target frequency.
- [x] **Orchestrator:** Smart chaining that automatically triggers Phase 2 after Phase 1 completes.
- [x] **State Locking:** Prevents overlapping scans on the same target.

---

## 🔜 Next Steps (Phase 3)

We are now transitioning to **Phase 3: Vulnerability Scanning**.

**Upcoming Goals:**
* Integrate **Nuclei** for template-based vulnerability scanning.
* Implement **Smart Filtering** (e.g., run WordPress templates only on WordPress assets).
* Send **Critical/High** vulnerability alerts to Telegram immediately.
* Store vulnerability reports in the database.