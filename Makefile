# docket developer tasks.
#   make install        compile the CLI/TUI into $(GOBIN) and link it as `docket` in $(LINK_DIR) (on PATH)
#   make test           vet + tests
#   make desktop-deps   install frontend packages + generate Wails JS bindings
#   make desktop-dev    run the desktop app with hot reload
#   make desktop        produce desktop/build/bin/Docket.app
#   make desktop-open   produce and open the .app

GOBIN    ?= $(shell go env GOPATH)/bin
LINK_DIR ?= $(HOME)/.local/bin
WAILS    ?= $(GOBIN)/wails

.PHONY: install test uninstall desktop desktop-deps desktop-dev desktop-open desktop-placeholder

install:
	go install .
	mkdir -p $(LINK_DIR)
	ln -sf $(GOBIN)/docket $(LINK_DIR)/docket
	@echo "docket -> $$(readlink $(LINK_DIR)/docket)"
	@command -v docket >/dev/null || echo "warning: $(LINK_DIR) is not on PATH"

test: desktop-placeholder
	go vet ./...
	go test ./...

uninstall:
	rm -f $(LINK_DIR)/docket $(GOBIN)/docket

# go:embed needs desktop/frontend/dist to exist even before the first frontend compile.
desktop-placeholder:
	@mkdir -p desktop/frontend/dist && touch desktop/frontend/dist/.gitkeep

desktop-deps: desktop-placeholder
	cd desktop/frontend && npm install
	cd desktop && $(WAILS) generate module

desktop-dev: desktop-deps
	cd desktop && $(WAILS) dev

desktop: desktop-deps
	cd desktop && $(WAILS) build -clean -platform darwin/universal

desktop-open: desktop
	open desktop/build/bin/Docket.app
