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

**示例：**
```bash
# 监听 3000 端口，允许列出文件，并设置 1 小时浏览器缓存
stasrv --port=3000 --enable-list-files --cache-age=3600 --location=/docs:./documents
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
