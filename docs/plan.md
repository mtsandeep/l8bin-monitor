# 📄 Litebin VPS Live Stats Agent – Final Plan

---

## 1. 🎯 Objective
Build a **lightweight, standalone agent** that runs on a VPS and exposes **live system + Docker stats** via HTTP, to be consumed by a demo/landing page application.

### Key Goals
* Real-time “live infra” feel
* Extremely low resource usage (<10MB RAM)
* Zero runtime external dependencies (compiled directly into language binary)
* Simple to deploy on any VPS

---

## 2. 🧱 High-Level Architecture & Tech Stack

* **Language**: Go (Golang) - chosen for simple binary distribution and minimal RAM footprint.
* **HTTP Server**: Go's standard library `net/http` to serve JSON.
* **System Stats Collection**: `shirou/gopsutil` package (fast and reliable without us reinventing the wheel).
* **Docker Stats Collection**: Direct HTTP requests to the Docker Unix Socket (`/var/run/docker.sock`) to circumvent the heavy official Docker SDK.
* **Cache Layer**: In-memory struct guarded by a Mutex, updated lazily (only when clients request data/stream) with a configurable interval.

```text
[VPS HOST]

  litebin-agent (Go binary) -> Lazy Ticker (active only when needed)
        ↓
  reads system (gopsutil) + docker stats (socket)
        ↓
  updates in-memory cache
        ↓
  exposes HTTP API (127.0.0.1:5008)
        ↓
  demo app (Docker container) fetches data instantly
```

---

## 3. ⚙️ Core Requirements

### Functional

* Provide **System stats**:
  * CPU usage (%)
  * RAM usage (used / total)
  * Disk usage (Aggregate of all physical partitions)
  * Load average

* Provide **Docker stats**:
  * List running containers
  * CPU usage per container
  * Memory usage per container
  * Container status

### Non-Functional

* RAM usage: **<10MB**
* Response time: **<10ms** (Served directly from memory cache)
* Runs continuously (daemon)
* Works on low-end VPS (512MB)

---

## 4. 🔌 API Design

### Base URL
```text
http://127.0.0.1:5008
```

### 4.1 `/stats`
```json
{
  "cpu": 21.5,
  "ram": {
    "used": 180,
    "total": 512
  },
  "disk": {
    "used": 4.2,
    "total": 20
  },
  "load": [0.12, 0.08, 0.05]
}
```

### 4.2 `/containers`
```json
[
  {
    "id": "abc123",
    "name": "demo-app",
    "status": "running",
    "cpu": 2.5,
    "memory": 45
  }
]
```

### 4.3 `/all` (Recommended)
Combines the output of `/stats` and `/containers` under one payload, adding a `timestamp` field.

### 4.4 `/stream` (Server-Sent Events)
A continuous open connection that pushes the `/all` payload every 1 second.
Perfect for zero-overhead streaming. Your frontend can connect via `new EventSource()` directly through a reverse proxy (like Nginx/Caddy), completely bypassing Node.js!

---

## 5. 🐳 Docker Integration Details

We will communicate with the Docker daemon natively via its socket API.

**Socket Path**: `/var/run/docker.sock`

**Internal Requests:**
1. `GET /containers/json`
2. `GET /containers/{id}/stats?stream=false`

---

## 6. 🚀 Deployment

### Binary Location
`/usr/local/bin/litebin-agent`

### systemd Service
```ini
[Unit]
Description=Litebin Stats Agent
After=docker.service

[Service]
ExecStart=/usr/local/bin/litebin-agent --port 5008
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
```

---

## 7. ⚡ Performance Strategy (Lazy Collection / Caching)
**Caching is critical during traffic bursts, but idling should consume zero resources.** 
* **On-Demand / Lazy:** Stats collection only triggers when an HTTP request is received, or if a `/stream` is active.
* **Shared Ticker for Streams:** When at least 1 client is connected to `/stream`, a background loop runs at a configurable interval (default: `1s`) to share one stat payload among all clients. When the last client disconnects, the loop stops entirely.

---

## 8. 🔒 Security
* **MUST**: The server must bind exclusively to `127.0.0.1:5008` so it is not exposed on the generic public IP network interface.

---

## 9. ✅ MVP Scope
* Handlers for `/stream`, `/all`, `/stats`, and `/containers`.
* Smart lazy-collection mechanism (zero idle CPU usage).
* Configurable interval via command-line flags.
* systemd deployment support.
