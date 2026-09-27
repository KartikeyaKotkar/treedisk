# treedisk: Go build, test, and install.

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
APPDIR ?= $(PREFIX)/share/applications
ICONDIR ?= $(PREFIX)/share/icons/hicolor/scalable/apps

GO ?= go
BINARY = bin/treedisk
ICON = assets/disktree.svg
DESKTOP = packaging/disktree.desktop.in

.PHONY: help build run install uninstall test clean go-install

help:
	@echo "treedisk (Go)"
	@echo
	@echo "  make build        build release binary to bin/treedisk"
	@echo "  make go-install   run 'go install' to install treedisk into GOBIN"
	@echo "  make run          build and run, scanning $$HOME"
	@echo "  make test         run all unit tests"
	@echo "  make install      install binary to $(PREFIX)"
	@echo "  make uninstall    remove installed binary and desktop entry"
	@echo "  make clean        remove build artifacts"

build:
	@mkdir -p bin
	$(GO) build -ldflags="-s -w" -o $(BINARY) .

run: build
	./$(BINARY)

test:
	$(GO) test -v ./...

clean:
	rm -rf bin

go-install:
	$(GO) install -ldflags="-s -w" .

install: build
	install -d $(BINDIR) $(APPDIR) $(ICONDIR)
	install -m755 $(BINARY) $(BINDIR)/treedisk
	ln -sf $(BINDIR)/treedisk $(BINDIR)/disktree
	install -m644 $(ICON) $(ICONDIR)/treedisk.svg
	sed -e 's|@BINDIR@|$(BINDIR)|' -e 's|@VERSION@|0.1.0|' \
	    $(DESKTOP) > $(APPDIR)/treedisk.desktop && \
	chmod 644 $(APPDIR)/treedisk.desktop
	@if command -v update-desktop-database >/dev/null 2>&1; then \
	    update-desktop-database $(APPDIR) 2>/dev/null || true; \
	fi
	@echo "installed $(BINDIR)/treedisk"

uninstall:
	rm -f $(BINDIR)/treedisk $(BINDIR)/disktree
	rm -f $(APPDIR)/treedisk.desktop
	rm -f $(ICONDIR)/treedisk.svg
	@echo "removed $(BINDIR)/treedisk"
