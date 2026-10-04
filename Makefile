.PHONY: install

# install builds lucind-ai with a real, traceable version string, installs it
# to $GOBIN (or $GOPATH/bin, already on PATH), then installs the agy plugin and links the Claude skill
# so the binary and the plugin hooks (which embed the binary's absolute path)
# never drift. Run this after any change to the binary instead of building to
# an ad-hoc temp path.
GOBIN_DIR := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)

install:
	go install -ldflags "-X main.version=$$(git describe --tags --always --dirty)" ./cmd/lucind-ai
	$(GOBIN_DIR)/lucind-ai plugin install
	mkdir -p $(HOME)/.claude/skills
	ln -sfn $(CURDIR)/plugin/claude-code/skills/lucind $(HOME)/.claude/skills/lucind
