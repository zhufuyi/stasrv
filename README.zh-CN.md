## [English](README.md) | 中文

<div align="center">

[![Go Report](https://goreportcard.com/badge/github.com/zhufuyi/stasrv)](https://goreportcard.com/report/github.com/zhufuyi/stasrv)
[![codecov](https://codecov.io/gh/zhufuyi/stasrv/branch/main/graph/badge.svg)](https://codecov.io/gh/zhufuyi/stasrv)
[![Go Reference](https://pkg.go.dev/badge/github.com/zhufuyi/stasrv.svg)](https://pkg.go.dev/github.com/zhufuyi/stasrv)
[![CI](https://github.com/zhufuyi/stasrv/actions/workflows/ci.yml/badge.svg)](https://github.com/zhufuyi/stasrv/actions)
[![License: MIT](https://img.shields.io/github/license/zhufuyi/stasrv)](https://github.com/zhufuyi/stasrv/blob/main/LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/zhufuyi/stasrv)](https://github.com/zhufuyi/stasrv/releases)

</div>

---

## 概述

`stasrv` 是一个基于 [Hertz](https://github.com/cloudwego/hertz) 构建的轻量级、高性能静态文件服务器。它可以作为独立服务运行，轻松替代 Nginx 来托管前端静态资源（HTML、CSS、JS、图片等），特别适合微服务架构、容器化部署或本地开发场景。

### 特性

-   **零依赖部署**：编译为单一可执行文件，无运行时依赖，即插即用。
-   **灵活路由映射**：支持多个 `path:root` 映射，可轻松挂载到不同的子路径。
-   **高性能引擎**：基于 CloudWeGo 的 Hertz 框架，具备极高的并发处理能力和极低的资源占用。
-   **内置缓存优化**：支持针对 JS、CSS、图片、字体等资源的 `Cache-Control` 设置。
-   **自适应 gzip 压缩**：根据目录权限自适应开启gzip预压缩。
-   **文件上传与删除接口**：通过 `POST <path>/upload` 接收文件并保存到 location 的 `data` 目录，可用 `DELETE <path>/delete/<文件>` 删除。
-   **静态资源嵌入**：支持将所有静态文件直接编译进二进制文件，实现真正的“单文件分发”。
-   **Docker 友好**：提供官方镜像，支持快速容器化部署。

## 安装

### 使用 Go 安装
```bash
go install github.com/zhufuyi/stasrv/cmd/stasrv@latest
```
*请确保 `$GOPATH/bin` 已添加到系统的 PATH 中。*

### 下载二进制文件
直接从 [Releases](https://github.com/zhufuyi/stasrv/releases) 页面下载适用于您系统的预编译二进制文件。

## 快速开始

```bash
# 将本地的 ./dist 目录映射到根路径 /
stasrv --location=/my-app:./dist

# 映射多个路径
#stasrv --location=/app1:./dist1 --location=/assets:./static
```

启动后，访问 `http://localhost:8080/my-app` 即可查看您的 index.html 页面内容。

## 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `--location` | string | - | `path:root` 格式的映射（支持多次使用以配置多个路由） |
| `--port` | int | `8080` | HTTP 服务监听端口 |
| `--enable-list-files` | bool | `false` | 是否允许列出目录下的文件列表 |
| `--cache-age` | int | `0` | 静态资源缓存时间（秒），0 表示不缓存 |
| `--fs-base-path` | string | - | 开启嵌入文件功能后的访问基础路径 |
| `--upload-max-size` | int | `32` | 单个上传文件以及上传请求体的大小上限，单位 MB |

**示例：**
```bash
# 监听 3000 端口，允许列出文件，并设置 1 小时浏览器缓存
stasrv --port=3000 --enable-list-files --cache-age=3600 --location=/docs:./documents
```

## 文件上传与删除接口

每个 location 在静态文件之外都会提供上传和删除两个接口，无需额外开关，`--location` 与
`--fs-base-path` 两种方式启动的服务都一样。路由前缀与该 location 的 `path` 一致：

| location | 上传路由 | 删除路由 | 保存目录 | 访问地址 |
| :--- | :--- | :--- | :--- | :--- |
| `--location=/docs:/app/dist` | `POST /docs/upload` | `DELETE /docs/delete/<文件>` | `/app/dist/data` | `/docs/data/<文件>` |
| `--location=/:./dist` | `POST /upload` | `DELETE /delete/<文件>` | `./dist/data` | `/data/<文件>` |
| `--fs-base-path=/embed` | `POST /embed/upload` | `DELETE /embed/delete/<文件>` | `<可执行文件所在目录>/data` | `/embed/data/<文件>` |

嵌入到二进制中的文件是只读的，所以 `--fs-base-path` 类型收到的文件保存在 `stasrv` 可执行文件同级的
`data` 目录，该目录在启动时创建。

### 上传

文件放在表单字段 `file` 中；同一个请求重复提交该字段即可一次上传多个文件。文件只保留客户端送来的
文件名最后一段（路径部分会被丢弃），同名文件会被覆盖。

```bash
# 上传单个文件，例如 data.json、report.pdf、picture.jpg
curl -F "file=@data.json" http://localhost:8080/docs/upload

# 一次上传多个文件
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

上传完成后即可通过返回的 `url` 直接访问该文件，缓存策略与其他静态文件一样受 `--cache-age` 控制。

返回码：`200` 上传成功，`400` 不是 multipart 请求 / 缺少 `file` 字段 / 文件名不可用，
`413` 超过 `--upload-max-size` 限制，`500` 文件写入失败。

### 删除

一个请求删除一个文件，文件名写在 URL 里：

```bash
curl -X DELETE http://localhost:8080/docs/delete/data.json
```

```json
{
  "name": "data.json",
  "deleted": true
}
```

文件名只在 `data` 目录内解析，因此该接口删不掉 location 自身的静态文件，目录也不会被删。

返回码：`200` 删除成功，`400` 文件名不可用 / 该名字是一个目录，`404` 文件不存在。

> **注意：** 两个接口都没有鉴权，并且会向挂载的目录写文件，请只在可信网络中使用，或放在有限制访问的
> 反向代理之后。服务的请求体上限会一并提升为 `--upload-max-size`，因为上传的文件就在请求体里。

仓库自带一个测试页面 `web/dist/index.html`，它会按自身所在的 URL 前缀推导出上传接口、删除接口和文件
地址，并在删除后回访文件地址确认已返回 404：

```bash
go run ./cmd/stasrv --location=/test:./web/dist
# 浏览器打开 http://localhost:8080/test/ 即可上传文件、读取内容并删除文件
```

## Docker 部署

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
      - ./dist:/app/dist:ro       # 与 location 参数配合使用
    command:
      # 设置`path:root`格式的静态资产映射（支持多个location）
      - --location=/my-app:/app/dist
      #- --location=/my-app2:/app/dist2
      #- --cache-age=2592000   # 缓存30天

    ports:
      - 8080:8080
```

## 嵌入静态文件

如果您希望将静态资源打包进二进制文件中（例如为了分发方便），请按以下步骤操作：

1.  **准备文件**：将静态资源放入源码的 `cmd/stasrv/static_dir` 目录下。
2.  **编译**：在项目根目录运行 `make build`。
3.  **运行**：启动时使用 `--fs-base-path` 参数指定访问路径。
    ```bash
    ./stasrv --fs-base-path=/ui
    ```
    此时，嵌入的文件将通过 `http://localhost:8080/ui` 访问。

## 与 Nginx 的对比

| 维度 | stasrv | Nginx |
| :--- | :--- | :--- |
| **安装体积** | ~10 MB (单文件) | 数十 MB + 依赖库 |
| **配置难度** | 极简 (命令行参数) | 较复杂 (需编写 nginx.conf) |
| **部署便捷性** | 极高 (支持文件嵌入) | 一般 (需同步静态目录) |
| **缓存控制** | 一键设置 `max-age` | 需配置 `expires` 或 `add_header` |
| **适用场景** | 微服务、CI/CD 预览、本地开发 | 复杂反向代理、高并发网关 |

## 开源协议
本项目基于 [MIT License](LICENSE) 协议开源。
