NAME := highrise-code
VERSION ?= 0.1.0
PREFIX ?= /usr
DESTDIR ?=
BINDIR := $(PREFIX)/bin
MANDIR := $(PREFIX)/share/man/man1
BASH_DIR := $(PREFIX)/share/bash-completion/completions
ZSH_DIR := $(PREFIX)/share/zsh/site-functions
FISH_DIR := $(PREFIX)/share/fish/vendor_completions.d
LICENSE_DIR := $(PREFIX)/share/licenses/$(NAME)
APPS_DIR := $(PREFIX)/share/applications
ICON_DIR := $(PREFIX)/share/icons/hicolor/scalable/apps
GO ?= go
GOFLAGS_BUILD := -trimpath -buildmode=pie -mod=readonly
LDFLAGS := -s -w -X github.com/ptitlu42/highrise-code/internal/cli.Version=$(VERSION)

.PHONY: all build test vet fmt check install uninstall clean

all: build

build:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS_BUILD) -ldflags "$(LDFLAGS)" -o bin/$(NAME) ./cmd/$(NAME)

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -l -w .

check: vet test

install: build
	install -Dm755 bin/$(NAME) $(DESTDIR)$(BINDIR)/$(NAME)
	install -Dm644 man/$(NAME).1 $(DESTDIR)$(MANDIR)/$(NAME).1
	install -Dm644 completions/$(NAME).bash $(DESTDIR)$(BASH_DIR)/$(NAME)
	install -Dm644 completions/_$(NAME) $(DESTDIR)$(ZSH_DIR)/_$(NAME)
	install -Dm644 completions/$(NAME).fish $(DESTDIR)$(FISH_DIR)/$(NAME).fish
	install -Dm644 packaging/desktop/$(NAME).desktop $(DESTDIR)$(APPS_DIR)/$(NAME).desktop
	install -Dm644 packaging/desktop/$(NAME).svg $(DESTDIR)$(ICON_DIR)/$(NAME).svg
	install -Dm644 LICENSE $(DESTDIR)$(LICENSE_DIR)/LICENSE

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(NAME)
	rm -f $(DESTDIR)$(MANDIR)/$(NAME).1
	rm -f $(DESTDIR)$(BASH_DIR)/$(NAME)
	rm -f $(DESTDIR)$(ZSH_DIR)/_$(NAME)
	rm -f $(DESTDIR)$(FISH_DIR)/$(NAME).fish
	rm -f $(DESTDIR)$(APPS_DIR)/$(NAME).desktop
	rm -f $(DESTDIR)$(ICON_DIR)/$(NAME).svg
	rm -rf $(DESTDIR)$(LICENSE_DIR)

clean:
	rm -rf bin dist
