# disktree: Go build, test, and install.

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
APPDIR ?= $(PREFIX)/share/applications
ICONDIR ?= $(PREFIX)/share/icons/hicolor/scalable/apps

GO ?= go
BINARY = bin/disktree
ICON = assets/disktree.svg
DESKTOP = packaging/disktree.desktop.in

.PHONY: help build run install uninstall test clean fmt

help:
	@echo "disktree (Go)"
	@echo
	@echo "  make build       build release binary"
	@echo "  make run         build and run, scanning $$HOME"
	@echo "  make test        run all unit tests"
	@echo "  make install     install binary to $(PREFIX)"
	@echo "  make uninstall   remove installed binary and desktop entry"
	@echo "  make clean       remove build artifacts"

build:
	@mkdir -p bin
	$(GO) build -ldflags="-s -w" -o $(BINARY) .

run: build
	./$(BINARY)

test:
	$(GO) test -v ./...

clean:
	rm -rf bin

install: build
	install -d $(BINDIR) $(APPDIR) $(ICONDIR)
	install -m755 $(BINARY) $(BINDIR)/disktree
	install -m644 $(ICON) $(ICONDIR)/disktree.svg
	sed -e 's|@BINDIR@|$(BINDIR)|' -e 's|@VERSION@|0.1.0|' \
	    $(DESKTOP) > $(APPDIR)/disktree.desktop && \
	chmod 644 $(APPDIR)/disktree.desktop
	@if command -v update-desktop-database >/dev/null 2>&1; then \
	    update-desktop-database $(APPDIR) 2>/dev/null || true; \
	fi
	@echo "installed $(BINDIR)/disktree"

uninstall:
	rm -f $(BINDIR)/disktree
	rm -f $(APPDIR)/disktree.desktop
	rm -f $(ICONDIR)/disktree.svg
	@echo "removed $(BINDIR)/disktree"
