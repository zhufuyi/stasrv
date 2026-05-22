## English | [中文](readme-cn.md)

<div align="center">

[![Go Report](https://goreportcard.com/badge/github.com/zhufuyi/stasrv)](https://goreportcard.com/report/github.com/zhufuyi/stasrv)
[![codecov](https://codecov.io/gh/zhufuyi/stasrv/branch/main/graph/badge.svg)](https://codecov.io/gh/zhufuyi/stasrv)
[![Go Reference](https://pkg.go.dev/badge/github.com/zhufuyi/stasrv.svg)](https://pkg.go.dev/github.com/zhufuyi/stasrv)
[![CI](https://github.com/zhufuyi/stasrv/actions/workflows/ci.yml/badge.svg)](https://github.com/zhufuyi/stasrv/actions)
[![License: MIT](https://img.shields.io/github/license/zhufuyi/stasrv)](https://github.com/zhufuyi/stasrv/blob/main/LICENSE)

</div>

---

`stasrv` is a lightweight static file server built on [Gin](https://github.com/gin-gonic/gin) and distributed as a single binary.  
It can run as a standalone service, easily replacing proxies like Nginx to host front-end static assets (HTML, CSS, JS, images, etc.). It is especially suitable for microservice architectures, containerized deployments, or local development scenarios.

## Features

- **Zero-Dependency Deployment**: Compiles into a single executable binary with no runtime dependencies.
- **Minimalist Configuration**: Simply specify the static file directory to get started.
- **Flexible Routing**: Supports custom URL base paths, making it easy to mount the service under a subpath.
- **Configurable Port**: Defaults to 8080, but can be changed to any port.
- **Production Ready**: Graceful shutdown, suitable for direct exposure or use behind a reverse proxy.
- **Lightweight & Efficient**: Powered by Gin's high-performance HTTP engine with minimal resource footprint.

## Installation

```bash
go install github.com/zhufuyi/stasrv/cmd/stasrv@latest
```

After installation, ensure that `$GOPATH/bin` is added to your system's `PATH` environment variable, then you can run the `stasrv` command directly.

Alternatively, you can download pre-compiled binaries from the [Releases](https://github.com/zhufuyi/stasrv/releases) page.

## Quick Start

```bash
# Specify the directory
stasrv --dir=/var/www/html
```

Open your browser and navigate to `http://localhost:8080` to see your `index.html` page.

## Command-Line Arguments

|**Argument**|**Type**|**Default**|**Description**|
|---|---|---|---|
|`--dir`|string|`.`|Path to the static file root directory (Required)|
|`--base-path`|string|`/`|Base URL path. For example, `/app` will mount files under `/app/`|
|`--cache-age`|int|`31536000`|Seconds for static asset `Cache-Control: max-age` (0 means no cache header)|
|`--port`|int|`8080`|HTTP service listening port|

File Caching Strategy:

- Images: Cached
- JS/CSS with hash: Cached
- JS/CSS without hash: Not cached
- HTML: Not cached

Example:

```bash
# Listen on port 3000, static directory set to ./public, base path set to /static, cache for 1 hour
stasrv --dir=./public --port=3000 --base-path=/static --cache-age=3600
```

You can now access your files via `http://localhost:3000/static/`.

## Docker Deployment

1. Run with Docker CLI

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/dist:/app/dist \
  zhufuyi/stasrv:latest \
  --dir=/app/dist --base-path=/app
```

2. Run with Docker Compose

```yaml
services:
  stasrv:
    image: zhufuyi/stasrv:latest
    restart: unless-stopped
    init: true
    volumes:
      - /etc/localtime:/etc/localtime:ro     # Host machine timezone
      - ./dist:/app/dist:ro                        # Map static assets
    command:
      - "--dir=/app/dist"      # Website static assets path
      - "--base-path=/app"  # URL prefix

    ports:
      - 8080:8080
```

Run `docker-compose up -d` to start the service.

Access `http://localhost:8080/app/` to view the `index.html` page.

## Comparison with Nginx

|**Scenario**|**stasrv**|**Nginx**|
|---|---|---|
|Installation Size|~10 MB single file|A few MBs + dependencies|
|Config Complexity|Single command|Requires writing `nginx.conf`|
|Dynamic Routing|Supported via `--base-path`|Configured via `location` directives|
|Cache Control|One-click `max-age` configuration|Requires manual `expires` headers|
|Best Used For|Microservice stasrv, local debugging, containerized environments|General reverse proxy, high concurrency scenarios|

`stasrv` is not intended to fully replace Nginx. Instead, it provides a **lighter, zero-configuration** alternative that significantly simplifies deployment in scenarios where complex reverse proxy rules are not required.

## Contributing

Issues and Pull Requests are welcome!

If you have great ideas or find a bug, please join the discussion on [GitHub Issues](https://github.com/zhufuyi/stasrv/issues).
