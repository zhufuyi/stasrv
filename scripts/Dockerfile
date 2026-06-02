FROM golang:1.25-alpine AS builder

RUN apk add git upx

WORKDIR /build
COPY go.mod go.sum ./
#RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .

RUN set -eux; \
    SERVER_NAME="stasrv"; \
    BUILD_DIR="cmd/${SERVER_NAME}"; \
    VERSION="dev"; \
    COMMIT="none"; \
    BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ'); \
    \
    if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
        VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
        COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none"); \
    fi; \
    \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -o "${SERVER_NAME}" \
        -trimpath \
        -ldflags "\
        -w -s \
        -X main.version=${VERSION} \
        -X main.buildTime=${BUILD_TIME} \
        -X main.commit=${COMMIT}" \
        ./${BUILD_DIR}

#compressing binary files
RUN upx -5 /build/stasrv


FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 1000 app && adduser -S -D -H -u 1000 -G app app

WORKDIR /app

COPY --from=builder --chown=app:app /build/stasrv /app/stasrv

USER app

EXPOSE 8080

# in /app directory
ENTRYPOINT ["./stasrv"]
#CMD ["--static-dir", "/app/dist", "--base-path","/"]
