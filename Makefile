.PHONY: install

# install builds lucind-ai with a real, traceable version string and
# installs it to $GOPATH/bin (already on PATH), so `lucind-ai -v` always
# reflects the exact commit just built -- run this after any change to the
# binary instead of building to an ad-hoc temp path.
install:
	go install -ldflags "-X main.version=$$(git describe --tags --always --dirty)" ./cmd/lucind-ai
