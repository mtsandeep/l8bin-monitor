# 🚀 Litebin VPS Live Stats

A **lightweight, standalone monitor** that exposes real-time Node System (CPU, RAM, Disk, Load), Docker container telemetry, and Docker daemon process stats via Server-Sent Events (SSE), alongside a stunning, premium frontend dashboard built with Vite.

The goal is to provide a "live infra" feel with exactly **zero overhead** when there are no connected clients, achieving <10MB memory usage through statically linked Go binaries and UPX compression.

---

## 1. 🏗️ Architecture Stack

- **Backend (Monitor)**: Go (Golang) — Single compiled binary.
  - System Stats: `shirou/gopsutil/v3`
  - Docker Container Stats: Direct REST to Docker Socket (circumventing heavy official SDK logic)
  - Docker Daemon Processes: Host-level process enumeration (dockerd, containerd, docker-proxy, etc.) with RAM & swap tracking, grouped by name
  - Concurrency: Zero-allocation lazy Ticker (starts _only_ when a frontend attempts an SSE stream).
- **Frontend (Dashboard Demo)**: Vite + Vanilla JS + Glassmorphism CSS.

---

## 2. 💡 Why Go?

Although the rest of our stack is built in Rust, we chose Go for this specific monitor to prioritize **Development Simplicity** and **Distribution Velocity**:

1.  **Batteries-Included Standard Library**: Go handles HTTP/SSE and static asset embedding natively without large external crates like `tokio` or `axum`.
2.  **Goroutines for Lazy Streaming**: The "Zero Overhead" goal (starting telemetry only when a client connects) is trivially simple to implement with Go's CSP concurrency model.
3.  **True Static Linking**: `CGO_ENABLED=0` produces a truly standalone, single binary with zero dependencies, making it perfect for direct distribution (SCP/curl) to any Linux VPS.
4.  **Memory Efficiency**: Go's runtime and GC are highly optimized for this kind of "lazy" background telemetry.

> [!NOTE]
> **Performance Baseline**: Initial testing shows the monitor running at approximately **~15MB RAM** usage under active streaming. We consider this a solid baseline for the tool, and further optimizations will be evaluated as more real-world usage data becomes available.

---

## 3. 💻 Local Development on Windows

You can safely develop and test this on your Windows machine before deploying to your target Linux VPS! The monitor is cross-platform aware.

### 3.1: Enable Docker Desktop API (Windows Only)

Since Windows does not natively use `/var/run/docker.sock` like Linux, the monitor falls back to TCP.

1. Open **Docker Desktop**.
2. Go to **Settings -> General**.
3. Check the box for **"Expose daemon on tcp://localhost:2375 without TLS"**.
4. Click "Apply & restart".

### 3.2: Start the Go Backend

Ensure you are in the project root containing `main.go`.

```bash
# Clean missing dependencies
go mod tidy

# Run the monitor natively on port 5008
go run . --port 5008 --interval 1s
*(The backend will securely bind to `http://127.0.0.1:5008`)*

# Check current version
go run . -v
```

### 3.3: Start the Vite Frontend Demo

Open a completely _new_ terminal window.

```bash
cd frontend
npm install
npm run dev
```

Open **`http://localhost:5173/`** in your browser to view the live dashboard!

---

## 4. 📦 Install & Update (Linux VPS)

The install script handles both **fresh installs** and **updates** — it stops the service if running, downloads the latest binary and service file, and starts it back up.

### 4.1: Quick Install (Auto-detect Architecture)

```bash
curl -fsSL https://raw.githubusercontent.com/mtsandeep/l8bin-monitor/main/scripts/install.sh | sudo bash
```

### 4.2: Manual Install

Pick the right binary for your platform:

| Platform | Asset Name |
| :--- | :--- |
| Linux x86-64 (most VPS) | `litebin-monitor-linux-amd64` |
| Linux ARM 64-bit (e.g. Oracle ARM) | `litebin-monitor-linux-arm64` |
| macOS Apple Silicon | `litebin-monitor-darwin-arm64` |
| macOS Intel | `litebin-monitor-darwin-amd64` |

```bash
# Download binary
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -fsSL "https://github.com/mtsandeep/l8bin-monitor/releases/latest/download/litebin-monitor-linux-${ARCH}" \
  -o /tmp/litebin-monitor
sudo mv /tmp/litebin-monitor /usr/local/bin/litebin-monitor
sudo chmod +x /usr/local/bin/litebin-monitor

# Install systemd service
curl -fsSL "https://raw.githubusercontent.com/mtsandeep/l8bin-monitor/main/litebin-monitor.service" \
  -o /tmp/litebin-monitor.service
sudo mv /tmp/litebin-monitor.service /etc/systemd/system/litebin-monitor.service
sudo systemctl daemon-reload
sudo systemctl enable litebin-monitor
sudo systemctl start litebin-monitor
```

### 4.3: Verify

```bash
litebin-monitor -v
# litebin-monitor v1.x.x
```

---

## 5. 🐧 Production Build & Release

The monitor is designed for high-portability. For production, we use **static linking** (`CGO_ENABLED=0`) and **UPX compression** to create a tiny, standalone binary that runs on any Linux distribution without dependencies.

### 5.1: Professional Build (Standard Workflow)

Using our automation scripts ensures you get consistent, optimized, and cross-platform binaries.

**Windows (PowerShell):**

```powershell
.\scripts\build.ps1 v1.0.0
```

**Unix/macOS (Bash):**

```bash
chmod +x ./scripts/build.sh
./scripts/build.sh v1.0.0
```

### 5.2: Automated GitHub Release (CI/CD)

The repository is pre-configured with a GitHub Action. Every time you push a **git tag**, it automatically builds all versions, compresses them with **UPX**, and creates a GitHub Release.

```bash
git tag v1.0.0
git push origin v1.0.0
```

Then, check the **"Releases"** section of your GitHub repository to download your production-ready binaries directly.

### 5.3: Manual Build (Single Platform)

If you just want to build for your current Linux system quickly:

```bash
# Build the compact binary
GOOS=linux go build -ldflags="-s -w" -o litebin-monitor .

# Move to standard bin (require sudo)
mv litebin-monitor /usr/local/bin/
```

---

## 6. ⚙️ Configuration

### 6.1: CLI Flags

| Flag              | Type       | Default      | Description                                               |
| :---------------- | :--------- | :----------- | :-------------------------------------------------------- |
| `--host`          | `string`   | `127.0.0.1`  | Host address to bind the HTTP server to.                  |
| `--port`          | `int`      | `5008`       | Port to run the HTTP server on.                           |
| `--interval`      | `duration` | `1s`         | Update interval (e.g., `500ms`, `2s`).                    |
| `--strip-prefix`  | `string`   | `""`         | Prefix to remove from container names (e.g., `litebin-`). |
| `--version`, `-v` | `bool`     | `false`      | Show the current version and exit.                        |

**Examples:**

```bash
# Default: localhost only
./litebin-monitor --port 5008

# Bind to Docker network gateway so containers can access it
./litebin-monitor --host 172.18.0.1 --port 5008

# With all options
./litebin-monitor --host 172.18.0.1 --port 8080 --interval 500ms --strip-prefix "prod-"
```

### 6.2: systemd Service

The install script sets this up automatically. To customize, edit the service file:

```bash
sudo systemctl edit litebin-monitor --full
```

Default service file:

```ini
[Unit]
Description=Litebin Stats Monitor
After=docker.service

[Service]
ExecStart=/usr/local/bin/litebin-monitor --host 172.18.0.1 --port 5008
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
```

### 6.3: Reverse Proxy (Nginx)

To securely expose the monitor to the public without exposing the raw port:

```nginx
location /stats/stream {
    proxy_pass http://127.0.0.1:5008/stream;
    proxy_set_header Connection '';
    proxy_http_version 1.1;
    chunked_transfer_encoding off;
    proxy_buffering off;
    proxy_cache off;
}
```
