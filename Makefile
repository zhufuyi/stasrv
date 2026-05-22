SHELL := /bin/bash

PROJECT_NAME := "github.com/zhufuyi/stasrv"
PKG := "$(PROJECT_NAME)"
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/ | grep -v /api/ | grep -v /cmd/)


.PHONY: ci-lint
# Run golangci-lint to check code formatting, naming, security, and maintainability.
ci-lint:
	@golangci-lint fmt ./...
	@golangci-lint run ./...


.PHONY: test
# Run unit tests (*_test.go). Disables test caching using -count=1.
test:
	go test -count=1 -short ${PKG_LIST}


.PHONY: cover
# Generate test coverage report and open HTML coverage view.
# Examples:
#   make cover
#   make cover HTML=false
#   make cover OUT=name.out
#   make cover HTML=false OUT=name.out
cover:
	@out="$(OUT)"; \
	if [ -z "$$out" ]; then out="cover.out"; fi; \
	html="$(HTML)"; \
	if [ -z "$$html" ]; then html="true"; fi; \
	if [ -z "$$out" ]; then echo "Error: OUT must not be empty"; exit 1; fi; \
	go test -short -coverprofile="$$out" -covermode=atomic ${PKG_LIST}; \
	if [ "$$html" = "true" ]; then go tool cover -html="$$out"; fi


.PHONY: build
# Build the stasrv binary. Supports cross-compilation.
# Examples:
#   make build
#   make build GOOS=linux GOARCH=amd64
build:
	@bash scripts/build.sh $(GOOS) $(GOARCH)


.PHONY: run
# Build and run the service locally.
# Parameters:
#   DIR      : Directory of static web files.
#   BASE_PATH: Base path of the service, default /.
#   PORT     : Port to run the service on, default 8080.
#   BUILD    : Build the service binary, default true.
# Examples:
#   make run DIR=/path/to/web/static/dir
#   make run DIR=/path/to/web/static/dir BASE_PATH=/url/prefix/path
#   make run DIR=/path/to/web/static/dir BUILD=false
run:
	@if [ -z "$(DIR)" ]; then \
		echo "Error: DIR is required. Usage: make run DIR=/path/to/web/static/dir"; \
		exit 1; \
	fi
	@MSYS_NO_PATHCONV=1 bash scripts/run.sh $(DIR) $(BASE_PATH)


.PHONY: docker-build
# Build multi-arch Docker image and (optionally) push to a registry.
# Parameters:
#   REGISTRY : Docker Hub username or private registry host/namespace.
#   TAG      : Image tag (e.g. v1.0.0).
#   PUSH     : Whether to push the image (true/false, default true).
# Examples:
#   make docker-build REGISTRY=john TAG=v1.0.0                        # build & push
#   make docker-build REGISTRY=host/namespace TAG=v1.0.0 PUSH=false   # build only, load locally
docker-build:
	@if [ -z "$(REGISTRY)" ] || [ -z "$(TAG)" ]; then \
		echo "ERROR: REGISTRY and TAG are required."; \
		echo "Usage: make docker-build REGISTRY=<registry> TAG=<tag> [PUSH=true|false]"; \
		exit 1; \
	fi
	@bash scripts/docker-build.sh $(REGISTRY) $(TAG) $(or $(PUSH),true)


.PHONY: docker-clean
# Clean up Docker resources.
# Usage:
#   make docker-clean                 # default: stopped containers + dangling images
#   make docker-clean NOCACHE=true    # + build cache
#   make docker-clean ALL=true        # full system prune (all unused images, containers, networks, cache)
docker-clean-image:
	@bash scripts/docker-clean.sh


# Print help information
help:
	@echo ''
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk '\
	function print_help(cmd, text) { \
		n = split(text, lines, "\n"); \
		printf "\033[1;36m  %-22s\033[0m %s\n", cmd, lines[1]; \
		for (i = 2; i <= n; i++) { \
			printf "  %-22s %s\n", "", lines[i]; \
		} \
		printf "\n" \
	} \
	/^# / { \
		line=$$0; sub(/^# /,"",line); \
		if (help=="") help=line; \
		else help=help"\n"line; \
		next; \
	} \
	/^\.PHONY:/ { \
		help=""; \
		next; \
	} \
	/^[a-zA-Z0-9_.-]+:/ { \
		cmd=$$1; sub(/:.*/,"",cmd); \
		if (help!="") { \
			print_help(cmd, help); \
			help=""; \
		} \
		next; \
	} \
	{ help="" } \
	' $(MAKEFILE_LIST)

.DEFAULT_GOAL := all
