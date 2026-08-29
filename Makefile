# Everything runs in a container, so the only host requirement is docker.
# Every directory with a go.mod is a module. Override to work on one:
#   make test MODULES=graceful
MODULES ?= $(patsubst %/go.mod,%,$(wildcard */go.mod))

# Pinned: an unpinned toolchain silently changes what CI accepts.
# GO_IMAGE must be >= the highest `go` directive across the modules.
GO_IMAGE   ?= golang:1.27-bookworm
LINT_IMAGE ?= golangci/golangci-lint:v2.13.2
DOCKER     ?= docker

# Caches live in their own host-owned dirs rather than the host GOPATH: a named
# volume would be root-owned and unwritable for the unprivileged user we run as,
# and reusing the host cache couples container builds to the host Go config.
CACHE_DIR      := $(HOME)/.cache/liquidcats-libraries
GOCACHE_DIR    := $(CACHE_DIR)/build
GOMODCACHE_DIR := $(CACHE_DIR)/mod
DEPS_STAMP     := .make/deps.stamp

# Default to no network. Only dependency download opts back in, so a compromised
# dependency cannot phone home from a test, a build step or a linter plugin.
NETWORK ?= none

# --user: artifacts (go.sum, --fix rewrites) stay owned by the caller, not root.
# --read-only + tmpfs: nothing outside /src and the caches is writable.
# GOTOOLCHAIN=local: fail loudly on a version bump instead of fetching a toolchain.
# GOFLAGS is left at the default -mod=readonly so no target silently edits go.mod.
DOCKER_RUN = $(DOCKER) run --rm \
	--user $(shell id -u):$(shell id -g) \
	--network=$(NETWORK) \
	--cap-drop=ALL \
	--security-opt=no-new-privileges \
	--read-only \
	--tmpfs /tmp:rw,nosuid,nodev,exec,size=2g \
	--volume "$(CURDIR):/src" \
	--volume "$(GOCACHE_DIR):/gocache" \
	--volume "$(GOMODCACHE_DIR):/gomodcache" \
	--env GOCACHE=/gocache \
	--env GOMODCACHE=/gomodcache \
	--env GOPATH=/tmp/gopath \
	--env GOTOOLCHAIN=local \
	--env HOME=/tmp

# Run a command in each module, stopping at the first failure.
# $(1) image + binary, $(2) arguments
define each
@mkdir -p "$(GOCACHE_DIR)" "$(GOMODCACHE_DIR)"
@for m in $(MODULES); do \
	echo "==> $$m"; \
	$(DOCKER_RUN) --workdir "/src/$$m" $(1) $(2) || exit 1; \
done
endef

GO   = $(GO_IMAGE) go
LINT = $(LINT_IMAGE) golangci-lint --config=/src/.golangci.yaml

.PHONY: all test bench lint lint-fix fmt tidy deps shell clean

all: test lint

test: $(DEPS_STAMP)
	$(call each,$(GO),test -race -vet=all ./...)

bench: $(DEPS_STAMP)
	$(call each,$(GO),test -run NONE -bench . -benchmem ./...)

lint: $(DEPS_STAMP)
	$(call each,$(LINT),run ./...)

lint-fix: $(DEPS_STAMP)
	$(call each,$(LINT),run --fix ./...)

fmt:
	$(call each,$(LINT),fmt ./...)

# The only targets allowed to reach the network.
$(DEPS_STAMP) deps tidy: NETWORK = bridge

# Warms the module cache so every other target can run with --network=none.
# Re-runs whenever a manifest changes; MODULES is ignored here on purpose.
$(DEPS_STAMP): $(wildcard */go.mod) $(wildcard */go.sum)
	@mkdir -p $(@D)
	@$(MAKE) --no-print-directory deps MODULES="$(patsubst %/go.mod,%,$(wildcard */go.mod))"
	@touch $@

deps:
	$(call each,$(GO),mod download)

tidy:
	$(call each,$(GO),mod tidy)

# Interactive poke-around, same sandbox. Needs a TTY, so it is not used above.
shell: $(DEPS_STAMP)
	$(DOCKER) run --rm -it --user $(shell id -u):$(shell id -g) \
		--network=$(NETWORK) --cap-drop=ALL --security-opt=no-new-privileges \
		--tmpfs /tmp:rw,nosuid,nodev,exec,size=2g \
		--volume "$(CURDIR):/src" \
		--volume "$(GOCACHE_DIR):/gocache" --volume "$(GOMODCACHE_DIR):/gomodcache" \
		--env GOCACHE=/gocache --env GOMODCACHE=/gomodcache \
		--env GOTOOLCHAIN=local --env HOME=/tmp \
		--workdir /src $(GO_IMAGE) bash

clean:
	rm -rf .make
	chmod -R u+w "$(GOMODCACHE_DIR)" 2>/dev/null || true
	rm -rf "$(CACHE_DIR)"
