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

`stasrv` 是一个基于 [Gin](https://github.com/gin-gonic/gin) 构建的轻量级静态文件服务器，以单个二进制文件分发。  
它可以作为独立服务运行，轻松替代 Nginx 等代理来托管前端静态资源（HTML、CSS、JS、图片等），尤其适合微服务架构、容器化部署或本地开发场景。

## 特性

- **零依赖部署**：编译为单一可执行文件，无运行时依赖。
- **极简配置**：只需指定静态文件目录即可启动。
- **灵活路由**：支持自定义 URL 基础路径，方便挂载在子路径下。
- **端口可配**：默认 8080，可随意更改。
- **生产可用**：优雅关闭，适合直接暴露或配合反向代理使用。
- **轻量高效**：基于 Gin 的高性能 HTTP 引擎，资源占用极低。

## 安装

```bash
go install github.com/zhufuyi/stasrv/cmd/stasrv@latest
```

安装完成后，确保 `$GOPATH/bin` 在系统 PATH 中，即可直接运行 `stasrv` 命令。

也可以从 [Releases](https://github.com/zhufuyi/stasrv/releases) 页面下载预编译的二进制文件。

## 快速开始

```bash
# 指定目录
stasrv --dir=/var/www/html
```

浏览器访问 `http://localhost:8080`，即可看到静态文件列表或 `index.html` 页面。

## 命令行参数

| 参数 | 类型 | 默认值  | 说明                                         |
|------|------|------|--------------------------------------------|
| `--dir` | string |      | 静态文件根目录的路径（必填）                             |
| `--base-path` | string | `/`  | URL 的基础路径，例如 `/app` 会将文件挂载在 `/app/` 下      |
| `--port` | int | `8080` | HTTP 服务监听端口                                |
| `--enable-list-files` | boolean     | `false`  | 允许访问文件列表                               |
| `--cache-age` | int | `0`  | 缓存JS、CSS和图像静态资源，单位为秒，0表示没有缓存 |

示例：

```bash
# 监听 3000 端口，静态目录为 ./dist，基础路径为 /app，允许访问文件列表
stasrv --dir=./dist --port=3000 --base-path=/app --enable-list-files
```

此时可通过 `http://localhost:3000/static/` 访问文件。

## Docker 部署

1. 运行 docker 镜像

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/dist:/app/dist \
  zhufuyi/stasrv:latest \
  --dir=/app/dist --base-path=/app
```

2. docker-compose 启动

```yaml
services:
  stasrv:
    image: zhufuyi/stasrv:latest
    restart: unless-stopped
    init: true
    volumes:
      - /etc/localtime:/etc/localtime:ro     # 宿主机时区
      - ./dist:/app/dist:ro                        # 映射静态资源
    command:
      - "--dir=/app/dist"      # 网站静态资源路径
      - "--base-path=/app"  # url 前缀

    ports:
      - 8080:8080
```

运行 `docker-compose up -d` 启动服务。

访问 `http://localhost:8080/app/` 即可访问 index.html 页面。

## 与 Nginx 的对比

| 场景 | stasrv | Nginx |
|------|----------|-------|
| 安装大小 | ~10 MB 单文件 | 数 MB + 依赖 |
| 配置复杂度 | 一条命令即可 | 需编写 nginx.conf |
| 动态路由 | 支持 `--base-path` | 通过 location 指令 |
| 缓存控制 | 一键设置 `max-age` | 需手工添加 expires 头 |
| 适用场景 | 微服务前端、本地调试、容器化 | 通用反向代理、高并发场景 |

`stasrv` 并非要取代 Nginx，而是提供一种**更轻量、无配置**的选择，在不需要复杂反向代理规则的场景下大幅简化部署。

## 贡献

欢迎提交 Issue 和 Pull Request！  
如果你有好的想法或发现了 bug，请到 [GitHub Issues](https://github.com/zhufuyi/stasrv/issues) 参与讨论。
