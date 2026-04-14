# 📄 Litebin VPS Live Stats Agent – Plan Document

---

## 1. 🎯 Objective

Build a **lightweight, standalone agent** that runs on a VPS and exposes **live system + Docker stats** via HTTP, to be consumed by a demo/landing page application.

### Key Goals

* Real-time “live infra” feel
* Extremely low resource usage (<10MB RAM)
* Independent from application stack (Node, etc.)
* Simple to deploy on any VPS

---

## 2. 🧱 High-Level Architecture

```text
[VPS HOST]

  litebin-agent (Go binary)
        ↓
  reads system + docker stats
        ↓
  exposes HTTP API (localhost only)
        ↓
  demo app (Docker container) fetches data
        ↓
  frontend polls and renders UI
```

---

## 3. ⚙️ Core Requirements

### Functional

* Provide system stats:

  * CPU usage (%)
  * RAM usage (used / total)
  * Disk usage
  * Load average

* Provide Docker stats:

  * Running containers
  * CPU usage per container
  * Memory usage per container
  * Container status

* Provide combined endpoint for simplicity

---

### Non-Functional

* RAM usage: **<10MB**
* Response time: **<10ms**
* Runs continuously (daemon)
* No external dependencies
* Works on low-end VPS (512MB)

---

## 4. 🔌 API Design

### Base URL

```text
http://127.0.0.1:7070
```

---

### 4.1 `/stats` (system only)

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

---

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

---

### 4.3 `/all` (recommended for frontend)

```json
{
  "system": { ... },
  "containers": [ ... ],
  "timestamp": 1710000000
}
```

---

## 5. 🧠 Internal Architecture (Agent)

### Components

#### 1. Collector Layer

* System collector
* Docker collector

#### 2. Cache Layer

* Stores last computed stats
* Refresh interval: **1 second**

#### 3. HTTP Server

* Serves cached data instantly

---

### Flow

```text
loop (every 1s):
  collect system stats
  collect docker stats
  update cache

HTTP request:
  return cached response
```

---

## 6. 🐳 Docker Integration

### Data Source

```text
/var/run/docker.sock
```

### Required APIs

* List containers:

```text
GET /containers/json
```

* Stats per container:

```text
GET /containers/{id}/stats?stream=false
```

---

### Requirements

* Agent must run with access to Docker socket
* No need for Docker CLI

---

## 7. 🚀 Deployment

### 7.1 Binary Placement

```bash
/usr/local/bin/litebin-agent
```

---

### 7.2 systemd Service

```ini
[Unit]
Description=Litebin Stats Agent
After=docker.service

[Service]
ExecStart=/usr/local/bin/litebin-agent --port 7070
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
```

---

### 7.3 Commands

```bash
sudo systemctl daemon-reload
sudo systemctl enable litebin-agent
sudo systemctl start litebin-agent
```

---

### 7.4 Verification

```bash
curl http://127.0.0.1:7070/all
```

---

## 8. 🌐 Integration with App (Docker)

### Run container with host access

```bash
--add-host=host.docker.internal:host-gateway
```

---

### Fetch inside app

```js
fetch("http://host.docker.internal:7070/all")
```

---

## 9. 🔁 Frontend Behavior

### Polling Strategy

* Interval: **1–2 seconds**

```js
setInterval(fetchStats, 2000)
```

---

### UI Elements

* CPU usage (progress bar)
* RAM usage
* Load average
* Container list:

  * name
  * memory usage
  * CPU usage
  * status

---

## 10. ⚡ Performance Strategy

### Caching (critical)

* Collect every 1s
* Serve cached response

### Why

* Avoid repeated Docker API calls
* Stable UI updates
* Lower CPU usage

---

## 11. 🔒 Security

### MUST

* Bind only to localhost:

```text
127.0.0.1:7070
```

### DO NOT

* Expose publicly
* Open port externally

---

## 12. 📦 CLI Design

Support multiple modes:

```bash
litebin-agent daemon   # default
litebin-agent once     # single run (debug)
litebin-agent serve    # HTTP only
```

---

## 13. 🧩 Extensibility (Future Scope)

Do NOT implement now, but design for:

* Logs streaming
* Remote commands
* Container restart
* Deploy hooks
* Multi-agent orchestration

---

## 14. ⚠️ Constraints & Tradeoffs

### Accepted

* Slight delay (1s) due to caching
* Approximate CPU stats (fine for UI)

### Avoid

* Real-time streaming complexity
* Heavy monitoring stack

---

## 15. ✅ MVP Scope (Strict)

### Build ONLY:

* `/all` endpoint
* system stats
* docker container stats
* 1-second cache loop
* systemd deployment

👉 Anything beyond this = v2

---

## 16. 🧠 Key Design Principles

* Keep it **stateless**
* Keep it **local-first**
* Keep it **small and predictable**
* Prefer **clarity over completeness**

---

## 17. 🚀 Success Criteria

You know it’s done when:

* Runs on 512MB VPS without noticeable load
* Shows live updating stats on landing page
* Lists containers correctly
* Feels “alive” to the user