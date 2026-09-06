BINARY_NAME = ccache-storage-redis
BINARY_NAME_UNIX = ccache-storage-redis+unix
BINARY_NAME_HTTPS = ccache-storage-rediss
INSTALL_DIR = /usr/local/bin
GO = CGO_ENABLED=0 go
LDFLAGS = -ldflags="-s -w"
BUILDFLAGS = -trimpath
SOURCES = $(wildcard *.go)

LINUX_ARCHS = amd64 arm64 riscv64 s390x
DARWIN_ARCHS = amd64 arm64
WINDOWS_ARCHS = amd64 arm64

LINUX_TARGETS = $(foreach x,$(LINUX_ARCHS),$(BINARY_NAME)-linux-$(x))
DARWIN_TARGETS = $(foreach x,$(DARWIN_ARCHS),$(BINARY_NAME)-darwin-$(x))
WINDOWS_TARGETS = $(foreach x,$(WINDOWS_ARCHS),$(BINARY_NAME)-windows-$(x).exe)
ALL_TARGETS = $(LINUX_TARGETS) $(DARWIN_TARGETS) $(WINDOWS_TARGETS)

.PHONY: all
all: build

.PHONY: build
build: $(BINARY_NAME)

$(BINARY_NAME): $(SOURCES)
	$(GO) mod download
	$(GO) build $(LDFLAGS) $(BUILDFLAGS) -o $@ .

.PHONY: build-debug
build-debug: $(BINARY_NAME)-debug

$(BINARY_NAME)-debug: $(SOURCES)
	$(GO) mod download
	$(GO) build -gcflags="all=-N -l" -o $@ .

.PHONY: build-all
build-all: $(ALL_TARGETS)

.PHONY: build-linux
build-linux: $(LINUX_TARGETS)

.PHONY: build-darwin
build-darwin: $(DARWIN_TARGETS)

.PHONY: build-windows
build-windows: $(WINDOWS_TARGETS)

$(BINARY_NAME)-linux-%: $(SOURCES)
	GOOS=linux GOARCH=$* $(GO) build $(LDFLAGS) $(BUILDFLAGS) -o $@ .

$(BINARY_NAME)-darwin-%: $(SOURCES)
	GOOS=darwin GOARCH=$* $(GO) build $(LDFLAGS) $(BUILDFLAGS) -o $@ .

$(BINARY_NAME)-windows-%.exe: $(SOURCES)
	GOOS=windows GOARCH=$* $(GO) build $(LDFLAGS) $(BUILDFLAGS) -o $@ .

.PHONY: install
install: build
	install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	ln -sf $(INSTALL_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME_UNIX)
	ln -sf $(INSTALL_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME_HTTPS)

.PHONY: uninstall
uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	rm -f $(INSTALL_DIR)/$(BINARY_NAME_UNIX)
	rm -f $(INSTALL_DIR)/$(BINARY_NAME_HTTPS)

.PHONY: clean
clean:
	rm -f $(BINARY_NAME)*
	rm -f THIRD_PARTY_LICENSES.txt
	$(GO) clean

.PHONY: licenses
licenses: THIRD_PARTY_LICENSES.txt

THIRD_PARTY_LICENSES.txt: go.mod go.sum
	command -v go-licenses >/dev/null 2>&1 || { echo "go-licenses not found. Install with: go install github.com/google/go-licenses@v1.6.0"; exit 1; }
	rm -rf .licenses_tmp
	go-licenses save . --save_path=.licenses_tmp --force
	echo "=== Dependency Licenses ===" > $@
	echo "" >> $@
	echo "" >> $@
	echo "---" >> $@
	echo "Module: Go Programming Language (Standard Library)" >> $@
	echo "License File: LICENSE" >> $@
	echo "---" >> $@
	if [ -f "$$(go env GOROOT)/LICENSE" ]; then \
	  cat "$$(go env GOROOT)/LICENSE" >> $@; \
	else \
	  curl -fsSL https://raw.githubusercontent.com/golang/go/master/LICENSE >> $@; \
	fi
	find .licenses_tmp -type f \( -name 'LICENSE*' -o -name 'COPYING*' \) 2>/dev/null | grep -v '/ccache/ccache-storage-redis/' | sort -u | while read -r licensefile; do \
	  relpath=$$(echo "$$licensefile" | sed 's|^.licenses_tmp/||'); \
	  modpath=$$(dirname "$$relpath"); \
	  echo "" >> $@; \
	  echo "---" >> $@; \
	  echo "Module: $$modpath" >> $@; \
	  echo "License File: $$(basename $$licensefile)" >> $@; \
	  echo "---" >> $@; \
	  cat "$$licensefile" >> $@; \
	done
	rm -rf .licenses_tmp

.PHONY: test
test:
	$(GO) test -v ./...

.PHONY: format
format:
	$(GO) fmt ./...
