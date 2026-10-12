# docket developer tasks.
#   make install   compile into $(GOBIN) and link it as `docket` in $(LINK_DIR) (on PATH)
#   make test      vet + tests

GOBIN    ?= $(shell go env GOPATH)/bin
LINK_DIR ?= $(HOME)/.local/bin

.PHONY: install test uninstall

install:
	go install .
	mkdir -p $(LINK_DIR)
	ln -sf $(GOBIN)/docket $(LINK_DIR)/docket
	@echo "docket -> $$(readlink $(LINK_DIR)/docket)"
	@command -v docket >/dev/null || echo "warning: $(LINK_DIR) is not on PATH"

test:
	go vet ./...
	go test ./...

uninstall:
	rm -f $(LINK_DIR)/docket $(GOBIN)/docket
