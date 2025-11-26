# 🎯 Professional Bug Bounty Hunting Platform

> A scalable, automated, and intelligent reconnaissance framework designed for security researchers and red teamers. Built with performance and continuous monitoring in mind.



## 💡 Core Philosophy

Unlike traditional, fire-and-forget scanner scripts, this platform is built as a **Continuous Hunting Machine**.

1.  **No-Waste Strategy:** We store everything. Even currently "dead" subdomains are monitored for future activation (Fresh Asset detection).
2.  **Smart Monitoring:** Distinguishes between existing assets and newly discovered ones.
3.  **Scalability:** Engineered to handle massive targets with hundreds of thousands of assets without crashing.

## 🏗️ Technical Architecture

The system is built on a modern, containerized microservice-like architecture:

* **Backend Core:** Golang (Fiber Framework) for high-performance APIs and concurrent workers.
* **Persistence:** PostgreSQL with GORM for structured data storage.
* **Job Queue:** Redis for managing long-running background reconnaissance tasks asynchronously.
* **Infrastructure:** Fully Dockerized environment with multi-stage builds for optimized images.

## 🛠️ Arsenal (Toolchain)

The platform integrates industry-standard Golang-based security tools within its isolated environment:

* **Discovery:** `subfinder`, `assetfinder`
* **Permutation/Mutation:** `alterx`
* **Validation/Resolution:** `puredns` (w/ massdns), `dnsx`
* **Probing (Phase 2 Ready):** `httpx`
* **Deep Scan:** `amass`

---

## 📊 Development Status: Phase 1 Complete

We are currently following a multi-phase development roadmap.

### ✅ Phase 1: Deep Recon & Discovery Engine (COMPLETED)

**Goal:** Build the core infrastructure and the initial discovery pipeline to find hidden and fresh assets.

**Achievements:**
- [x] Established full Docker/Go/Postgres/Redis infrastructure.
- [x] Implemented RESTful APIs for target management (`/api/targets`).
- [x] Developed an asynchronous worker for background scanning.
- [x] **Smart Recon Pipeline:** Implemented a full chain:
    1.  Passive Collection (Subfinder + Assetfinder)
    2.  Mutation Generation (Alterx)
    3.  Active Validation (Puredns)
- [x] **Smart Storage Logic:** Implemented "upsert" logic to track live/dead status and detect fresh assets.
- [x] **Data Retrieval APIs:** Implemented paginated and filterable APIs to view results (`is_live=true`, `is_new=true`).

---

## 🔜 Next Steps (Phase 2)

We are now transitioning to **Phase 2: Probing & Fingerprinting**.

**Upcoming Goals:**
* Utilize the live assets found in Phase 1.
* Run `httpx` to extract detailed technical information (Web Servers, Technologies, Titles, Status Codes, IPs).
* Update the database with rich fingerprinting data.