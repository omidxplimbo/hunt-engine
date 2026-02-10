# Changelog

All notable changes to the **Hunt Engine** project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v2.1.0] - 2026-02-10
### Added
- **Stability:** Added `Panic Recovery` mechanism in the worker dispatcher to prevent the entire worker process from crashing if a single job fails.
- **Reliability:** Implemented `clearAllLocks` on worker startup to automatically clean up stale lock files from previous crashes (fixing the "QUEUED forever" issue).
- **Intelligence:** Improved `VirusTotal` collection module with a proper User-Agent to mimic browser behavior and avoid potential API throttling or incomplete data.
- **Debugging:** Added detailed logging for VirusTotal URL discovery counts per domain.

## [v2.0.0] - 2026-02-09
### Added
- **Monitoring Server:** A new real-time monitoring dashboard exclusively for Admin users.
    - Visual Area Charts for CPU and RAM usage history.
    - Live "Active Processes" table showing command execution details (Target, Command, PID, Duration).
- **Backend Architecture:**
    - Integrated `gopsutil` for accurate system resource metrics.
    - Implemented `sync.Map` in the worker runner to thread-safely track and manage active process lifecycles.
    - New API endpoint `/api/monitor/stats` for fetching aggregated system and process data.
- **Frontend UI:**
    - New `MonitoringServer` component using `recharts` for visualization.
    - Integrated `TanStack Query` with polling (2s interval) for live updates without page refreshes.
    - Admin-protected visibility in the main Dashboard.

### Optimized
- **Performance:** Refactored command execution pipeline (specifically for `gau` and `waybackurls`) to use direct file I/O instead of piping through memory, significantly reducing RAM overhead during large scans.

### Fixed
- Dependency management: Resolved missing `go.sum` entries for system libraries.

## [v1.3.0]
### Added
- **Infrastructure:** Enhanced Docker composition and environment configuration.

## [v1.2.0]
### Added
- **Security:** IP-based access control middleware.
- **Security:** Enhanced authentication flows.

## [v1.1.0]
### Added
- **Scanning:** Integrated `waymore` tool for advanced URL discovery.
- **Scanning:** Enhanced crawling logic for better coverage.

## [v1.0.1]
### Fixed
- Bug fixes in crawling logic.
- Minor UI adjustments.

## [v1.0.0] - Initial Release
### Added
- **Core:** Complete Hunt Engine architecture (Backend + Frontend).
- **UI:** Cyberpunk-themed interface with responsive design.
- **Scanning:** Basic integration with subfinder, httpx, katana, nuclei.
- **Management:** Target management, User management (Admin/User roles).
