# Makefile for docker-zeek

BIN := zeek

# set version
VERSION ?= $(shell \
    git describe --tags --exact-match 2>/dev/null || \
    git describe --tags --dirty --always 2>/dev/null || \
    echo dev \
)

# set docker image tag
IMAGE_TAG := $(patsubst v%,%,$(VERSION))

# dev defaults for build/run
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# build flags
CGO_ENABLED ?= 0
LDFLAGS := -ldflags='-X main.Version=$(VERSION) -X main.ImageTag=$(IMAGE_TAG)'

RELEASE_TMP := .release_tmp
DOCKER_IMAGE := activecm/zeek

GOLANGCI_LINT ?= $(shell command -v golangci-lint-v2 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint-v2)

.PHONY: build test test-integration lint docker-build release-binaries release-images release-checksums release verify-release clean clean-release clean-all

# ----------------------
# dev build + run
# ----------------------
build:
	@echo "→ Building $(BIN) for $(GOOS)/$(GOARCH)..."
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BIN) .
	@echo "✔ Built $(BIN) version $(VERSION)"

test: build
	@echo "→ Running unit tests..."
	@go test -v -count=1 ./...
	@echo "✔ Unit tests passed"

test-integration: build
	@echo "→ Running integration tests (requires Docker)..."
	@go test -tags integration -v -count=1 -timeout 30m ./...
	@echo "✔ Integration tests passed"

lint:
	@echo "→ Running linter..."
	@$(GOLANGCI_LINT) run ./...
	@echo "✔ Lint passed"

# ----------------------
# docker
# ----------------------
docker-build:
	@echo "→ Building Docker image $(DOCKER_IMAGE):$(IMAGE_TAG)..."
	@docker build --build-arg VERSION=$(VERSION) -t $(DOCKER_IMAGE):$(IMAGE_TAG) .
	@echo "✔ Docker image built"

docker-save:
	@echo "→ Saving Docker image $(DOCKER_IMAGE):$(IMAGE_TAG)..."
	@docker save $(DOCKER_IMAGE):$(IMAGE_TAG) | gzip > zeek-image-offline-$(GOARCH).tar.gz
	@echo "✔ Saved zeek-image-offline-$(GOARCH).tar.gz"

# ----------------------
# local release artifacts (CI uses its own build steps with the git tag)
# ----------------------
release-binaries: clean-release
	@echo "→ Building release binaries..."
	@set -e; \
	for spec in linux/amd64 linux/arm64; do \
	  os="$${spec%/*}"; arch="$${spec#*/}"; \
	  name="$$arch"; \
	  echo "  → $(BIN) for $$os/$$arch..."; \
	  mkdir -p "$(RELEASE_TMP)/$$name"; \
	  env CGO_ENABLED=$(CGO_ENABLED) GOOS="$$os" GOARCH="$$arch" \
	    go build $(LDFLAGS) -o "$(RELEASE_TMP)/$$name/$(BIN)" .; \
	  tar -C "$(RELEASE_TMP)/$$name" -czf "zeek-linux-$$name.tar.gz" $(BIN); \
	done
	@echo "✔ Release binaries built"

release-checksums:
	@echo "→ Generating checksums..."
	@shasum -a 256 zeek-linux-*.tar.gz > checksums.txt
	@echo "✔ checksums.txt"

verify-release:
	@echo "→ Verifying release artifacts..."
	@set -e; \
	for name in amd64 arm64; do \
	  tarball="zeek-linux-$$name.tar.gz"; \
	  test -f "$$tarball" || (echo "✗ missing $$tarball" >&2; exit 1); \
	  tar -tzf "$$tarball" | grep -q "^$(BIN)$$" || (echo "✗ $$tarball does not contain $(BIN)" >&2; exit 1); \
	done
	@echo "→ Checking binary formats..."
	@set -e; \
	mkdir -p $(RELEASE_TMP)/verify; \
	for name in amd64 arm64; do \
	  tar -C "$(RELEASE_TMP)/verify" -xzf "zeek-linux-$$name.tar.gz"; \
	  if command -v file >/dev/null 2>&1; then \
	    case "$$name" in \
	      amd64) file "$(RELEASE_TMP)/verify/$(BIN)" | grep -Eqi "ELF.*(x86-64|x86_64)" || (echo "✗ $$name binary wrong arch" >&2; exit 1) ;; \
	      arm64) file "$(RELEASE_TMP)/verify/$(BIN)" | grep -Eqi "ELF.*(aarch64|arm64)" || (echo "✗ $$name binary wrong arch" >&2; exit 1) ;; \
	    esac; \
	  fi; \
	  rm -f "$(RELEASE_TMP)/verify/$(BIN)"; \
	done
	@rm -rf $(RELEASE_TMP)
	@echo "✔ Release verified"

release: release-binaries verify-release release-checksums
	@echo "✔ Release artifacts:"
	@ls -1 zeek-linux-*.tar.gz checksums.txt

# ----------------------
# clean
# ----------------------
clean:
	@rm -f $(BIN)

clean-release:
	@rm -rf $(RELEASE_TMP)
	@rm -f zeek-linux-*.tar.gz
	@rm -f zeek-image-offline-*.tar.gz
	@rm -f checksums.txt

clean-all: clean clean-release
