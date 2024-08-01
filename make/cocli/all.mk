FILE_IGNORES := $(FILE_IGNORES) \
	.vscode/ .DS_Store .idea/ bin/

GO_ALL_REPO_PKGS := ./cmd/... ./internal/... ./pkg/... ./api/...
GO_BINS := $(GO_BINS) \
	cmd/cocli
GOPRIVATE := $(GOPRIVATE)
GONOSUMDB := $(GONOSUMDB)
GOLANGCILINTTIMEOUT := 10m0s

include make/go/bootstrap.mk
include make/go/go.mk
include make/cocli/version.mk

.PHONY: build-binary
build-binary:
	go build -ldflags '${LDFLAGS}' -o ./bin/cocli ./cmd/cocli