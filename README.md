## English | [中文](README.zh-CN.md)

<div align="center">

[![Go Report](https://goreportcard.com/badge/github.com/zhufuyi/stasrv)](https://goreportcard.com/report/github.com/zhufuyi/stasrv)
[![codecov](https://codecov.io/gh/zhufuyi/stasrv/branch/main/graph/badge.svg)](https://codecov.io/gh/zhufuyi/stasrv)
[![Go Reference](https://pkg.go.dev/badge/github.com/zhufuyi/stasrv.svg)](https://pkg.go.dev/github.com/zhufuyi/stasrv)
[![CI](https://github.com/zhufuyi/stasrv/actions/workflows/ci.yml/badge.svg)](https://github.com/zhufuyi/stasrv/actions)
[![License: MIT](https://img.shields.io/github/license/zhufuyi/stasrv)](https://github.com/zhufuyi/stasrv/blob/main/LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/zhufuyi/stasrv)](https://github.com/zhufuyi/stasrv/releases)

</div>

---

## Overview

`stasrv` is a lightweight, high-performance static file server built on [Hertz](https://github.com/cloudwego/hertz). It serves as a modern alternative to Nginx for hosting frontend assets (HTML, CSS, JS, images, etc.), specifically designed for microservices, containerized environments, and local development.

### Features

-   **Zero-Dependency**: Compiles into a single binary with no runtime dependencies.
-   **Flexible Routing**: Supports multiple `path:root` mappings to mount assets under different sub-paths.
-   **High Performance**: Powered by CloudWeGo's Hertz framework, offering extreme concurrency and low resource footprint.
-   **Built-in Caching**: Easy `Cache-Control` configuration for JS, CSS, images, and fonts.
-   **Adaptive gzip compression**: Enable gzip pre-compression adaptively according to directory permissions.
-   **Asset Embedding**: Supports embedding static files directly into the binary using `go:embed` for "single-file deployment".
-   **Docker Ready**: Official lightweight images available for rapid deployment.

## Installation

### Via Go
```bash
go install github.com/zhufuyi/stasrv/cmd/stasrv@latest
```
*Ensure `$GOPATH/bin` is in your system's PATH.*

### Via Releases
Download the pre-compiled binaries for your platform from the [Releases](https://github.com/zhufuyi/stasrv/releases) page.

## Quick Start

```bash
# Map local ./dist directory to the root path /
stasrv --location=/:./dist

# Map multiple locations
stasrv --location=/app1:./dist1 --location=/assets:./static
```

Access your files at `http://localhost:8080/`.

## Command Line Arguments

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `--location` | string | - | Static asset mapping in `path:root` format (can be used multiple times) |
| `--port` | int | `8080` | Port to listen on |
| `--enable-list-files` | bool | `false` | Enable directory listing |
| `--cache-age` | int | `0` | Cache duration in seconds for assets (0 means no cache) |
| `--fs-base-path` | string | - | The base URL path when using embedded static files |

**Example:**
```bash
# Listen on port 3000, enable file listing, and set 1-hour browser cache
stasrv --port=3000 --enable-list-files --cache-age=3600 --location=/docs:./documents
```

## Docker Deployment

### Docker Run
```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/dist:/app/dist \
  zhufuyi/stasrv:latest \
  --location=/my-app:/app/dist
```

### Docker Compose
```yaml
services:
  stasrv:
    image: zhufuyi/stasrv:latest
    restart: unless-stopped
    init: true
    volumes:
      - /etc/localtime:/etc/localtime:ro
      - ./dist:/app/dist:ro       # Used with location parameter
    command:
      # Set static asset mapping in "path:root" format (multiple location supported)
      - --location=/my-app:/app/dist
      #- --location=/my-app2:/app/dist2
      #- --cache-age=2592000   # cache 30 days

    ports:
      - 8080:8080
```

## Embedding Static Files

To distribute your application as a single binary containing all assets:

1.  **Prepare Files**: Place your static assets into the `cmd/stasrv/static_dir` directory.
2.  **Build**: Run `make build` in the project root.
3.  **Run**: Use the `--fs-base-path` flag to specify the access path.
    ```bash
    ./stasrv --fs-base-path=/ui
    ```
    Your embedded files will be available at `http://localhost:8080/ui`.

## Comparison with Nginx

| Feature | stasrv | Nginx |
| :--- | :--- | :--- |
| **Binary Size** | ~10 MB (Single file) | Tens of MBs + Dependencies |
| **Configuration** | Simple (CLI Flags) | Complex (nginx.conf) |
| **Deployment** | Extremely Easy (Embedding) | Moderate (Syncing directories) |
| **Cache Control** | One-click `max-age` | Manual `expires` headers |
| **Best For** | Microservices, CI/CD, Dev | Complex Proxy, High-traffic Gateway |

## License
This project is licensed under the [MIT License](LICENSE).
