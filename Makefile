.PHONY: install verify-plugin-content verify-opencode-plugin install-opencode-plugin bump-plugin-version

# install builds lucind-ai with a real, traceable version string and
# installs it to $GOPATH/bin (already on PATH), so `lucind-ai -v` always
# reflects the exact commit just built -- run this after any change to the
# binary instead of building to an ad-hoc temp path.
install:
	go install -ldflags "-X main.version=$$(git describe --tags --always --dirty)" ./cmd/lucind-ai

# verify-plugin-content checks that plugin.json's version and the recorded
# skill-content hash (internal/packet/testdata/skill_content_hash.txt) still
# match the current plugin/claude-code/skills/lucind-ai/** tree. Run this
# deliberately -- e.g. right before actually publishing/releasing the
# plugin -- not as part of `make install` or `go test ./...`: a skill-tree
# content edit must never be force-coupled to a version bump on every
# ordinary commit (see internal/skillcontent's package doc for why).
verify-plugin-content:
	go run ./cmd/plugincontent verify

verify-opencode-plugin:
	go run ./cmd/plugincontent verify-opencode

install-opencode-plugin:
	./plugin/opencode/install.sh

# bump-plugin-version is the ONLY place a plugin version bump should
# originate from: it bumps plugin.json and marketplace.json to VERSION
# together and regenerates the recorded skill-content hash for the current
# tree. This is a deliberate, human-run action -- usage:
#   make bump-plugin-version VERSION=2.1.0
bump-plugin-version:
	@if [ -z "$(VERSION)" ]; then echo "usage: make bump-plugin-version VERSION=<new-version>" >&2; exit 2; fi
	go run ./cmd/plugincontent bump-version $(VERSION)
