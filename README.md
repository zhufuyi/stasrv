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
-   **File upload and delete APIs**: Receive files with `POST <path>/upload` into the `data` directory of a location, and remove them again with `DELETE <path>/delete/<file>`.
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
| `--upload-max-size` | int | `32` | Maximum size of an uploaded file and of the upload request body, unit is MB |

**Example:**
```bash
# Listen on port 3000, enable file listing, and set 1-hour browser cache
stasrv --port=3000 --enable-list-files --cache-age=3600 --location=/docs:./documents
```

## File Upload and Delete APIs

Every location serves an upload and a delete API on top of the static files, with no switch to
turn them on. Both routes reuse the location `path`:

| location | upload | delete | stored in | served at |
| :--- | :--- | :--- | :--- | :--- |
| `--location=/docs:/app/dist` | `POST /docs/upload` | `DELETE /docs/delete/<file>` | `/app/dist/data` | `/docs/data/<file>` |
| `--location=/:./dist` | `POST /upload` | `DELETE /delete/<file>` | `./dist/data` | `/data/<file>` |
| `--fs-base-path=/embed` | `POST /embed/upload` | `DELETE /embed/delete/<file>` | `<dir of the binary>/data` | `/embed/data/<file>` |

The embedded files themselves are read-only, so a `--fs-base-path` location keeps what it receives
in the `data` directory next to the `stasrv` executable, which is created on startup.

### Upload

Upload the files in the `file` form field; send the field several times to upload many files with
one request. A file is stored under its base name, so any path sent by the client is discarded, and
a file with the same name is overwritten.

```bash
# one file
curl -F "file=@data.json" http://localhost:8080/docs/upload

# several files in one request
curl -F "file=@data.json" -F "file=@report.pdf" -F "file=@picture.jpg" http://localhost:8080/docs/upload
```

```json
{
  "files": [
    {"name": "data.json", "size": 15, "url": "/docs/data/data.json"},
    {"name": "report.pdf", "size": 4096, "url": "/docs/data/report.pdf"}
  ]
}
```

An uploaded file is immediately available at the `url` returned above, and it follows the same
`--cache-age` rules as any other asset.

Responses: `200` stored, `400` not a multipart request / missing `file` field / unusable file name,
`413` larger than `--upload-max-size`, `500` the file could not be written.

### Delete

One request deletes one file, addressed by its name in the URL:

```bash
curl -X DELETE http://localhost:8080/docs/delete/data.json
```

```json
{
  "name": "data.json",
  "deleted": true
}
```

The name is always resolved inside the `data` directory, so the API cannot remove the assets of the
location itself, and a directory is never deleted.

Responses: `200` deleted, `400` unusable file name / the name is a directory, `404` no such file.

> **Note:** neither API has authentication and both write to the mounted volume, so keep them on
> trusted networks or put them behind a reverse proxy that restricts access. The server request body
> limit is raised to `--upload-max-size` too, because an upload carries the file in its body.

A ready-made test page ships in `web/dist`. It derives the routes from its own URL prefix and checks
that a deleted file answers `404` afterwards:

```bash
go run ./cmd/stasrv --location=/test:./web/dist
# then open http://localhost:8080/test/ to upload, read and delete files
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
