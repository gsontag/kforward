APP_ID=fr.gsontag.kforward

# Installation for the current user: no root needed
PREFIX?=$(HOME)/.local
BINDIR=$(PREFIX)/bin
APPDIR=$(PREFIX)/share/applications
ICONDIR=$(PREFIX)/share/icons/hicolor/scalable/apps

GOCMD=go
LINTCMD=golangci-lint
GOLANGCI_VERSION?=v2.14.0
GOLANGCI_MIN_VERSION?=2.13.0
GOVET=$(GOCMD) vet
BINARY_NAME=kforward
VERSION?=0.0.1

IT_CLUSTER?=kforward-it
IT_NODE_IMAGE?=kindest/node:v1.34.0
IT_APP_IMAGE?=busybox:stable
IT_KUBECONFIG?=$(CURDIR)/out/it-kubeconfig

I18N_DIR=internal/locale/translations
I18N_FLAGS=-sourceLanguage en -outdir $(I18N_DIR) -format toml

GREEN		:= $(shell tput -Txterm setaf 2)
YELLOW	:= $(shell tput -Txterm setaf 3)
WHITE		:= $(shell tput -Txterm setaf 7)
CYAN 		:= $(shell tput -Txterm setaf 6)
RESET		:= $(shell tput -Txterm sgr0)

## Tools
# N'installe que si la commande manque ou est trop ancienne : une version
# installée autrement (brew…) est conservée
install-tools: ## installs golangci-lint if missing or older than GOLANGCI_MIN_VERSION
		$(GOCMD) install github.com/nicksnyder/go-i18n/v2/goi18n@v2.6.1
		@current=$$($(LINTCMD) version --short 2>/dev/null); \
		if [ -n "$$current" ] && printf '%s\n%s\n' "$(GOLANGCI_MIN_VERSION)" "$$current" | sort -V -C; then \
			echo "$(LINTCMD) $$current found (>= $(GOLANGCI_MIN_VERSION))"; \
			exit 0; \
		fi; \
		echo "installing $(LINTCMD) $(GOLANGCI_VERSION) (found: $${current:-none})"; \
		$(GOCMD) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) || exit 1; \
		bindir=$$($(GOCMD) env GOBIN); bindir=$${bindir:-$$($(GOCMD) env GOPATH)/bin}; \
		current=$$($(LINTCMD) version --short 2>/dev/null); \
		if [ -z "$$current" ] || ! printf '%s\n%s\n' "$(GOLANGCI_MIN_VERSION)" "$$current" | sort -V -C; then \
			echo "warning: installed in $$bindir, but PATH resolves $(LINTCMD) to $${current:-nothing}"; \
			echo "         add $$bindir to PATH, before any older installation"; \
		fi

## Quality
check-quality: fmt-check tidy-check vet lint ## runs all quality checks, without modifying files

lint: ## go linting.
		$(LINTCMD) run

lint-fix: ## fix linting issues where possible
		$(LINTCMD) run --fix

lint-config: ## Verify golangci-lint linter configuration
		$(LINTCMD) config verify

vet: ## go vet
		$(GOVET) ./...

fmt: ## formats code with the formatters configured in .golangci.yml
		$(LINTCMD) fmt

fmt-check: ## fails if some code is not formatted
		$(LINTCMD) fmt --diff

tidy: ## runs tidy to fix go.mod dependencies
		$(GOCMD) mod tidy

tidy-check: ## fails if go.mod/go.sum are not tidy
		$(GOCMD) mod tidy -diff

## Translations
i18n-extract: ## collects the English texts, lists what is left to translate in translate.*.toml
		goi18n extract $(I18N_FLAGS) ./internal
		goi18n merge $(I18N_FLAGS) $(I18N_DIR)/active.*.toml

i18n-merge: ## moves the translated texts of translate.*.toml into active.*.toml
		goi18n merge $(I18N_FLAGS) $(I18N_DIR)/active.*.toml $(I18N_DIR)/translate.*.toml
		rm -f $(I18N_DIR)/translate.*.toml
		goi18n merge $(I18N_FLAGS) $(I18N_DIR)/active.*.toml

## Build
clean: ## cleans binary and other generated files
		$(GOCMD) clean
		rm -rf out/
		rm -f coverage*.out

build: ## Build binary
		$(GOCMD) build -o out/$(BINARY_NAME) .

## Install
install: build ## installs the application for the current user, in PREFIX
		install -Dm755 out/$(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)
		install -Dm644 data/$(APP_ID).svg $(ICONDIR)/$(APP_ID).svg
		install -d $(APPDIR)
		sed 's|@BINDIR@|$(BINDIR)|' data/$(APP_ID).desktop.in > $(APPDIR)/$(APP_ID).desktop
		chmod 644 $(APPDIR)/$(APP_ID).desktop

uninstall: ## removes what install added
		rm -f $(BINDIR)/$(BINARY_NAME) $(ICONDIR)/$(APP_ID).svg $(APPDIR)/$(APP_ID).desktop

## Test
# GTK packages build for minutes with -race: kept out of the unit and it tests
TEST_PKGS=$(shell $(GOCMD) list ./internal/... | grep -v /internal/gui)

test: ## runs tests and generates coverage report
		$(GOCMD) test -race -timeout 2m $(TEST_PKGS) -coverprofile=coverage.out

coverage: test ## displays test coverage report in html code
		$(GOCMD) tool cover -html=coverage.out

.PHONY: install-tools check-quality lint lint-fix lint-config vet fmt fmt-check tidy tidy-check \
	clean build test coverage all help it-cluster it-clean it install uninstall

## Integration tests
it-cluster: ## creates the kind cluster of the integration tests, if needed
		@if ! kind get clusters | grep -qx "$(IT_CLUSTER)"; then \
			kind create cluster --name "$(IT_CLUSTER)" --image "$(IT_NODE_IMAGE)" \
				--kubeconfig "$(IT_KUBECONFIG)" --wait 60s; \
		fi
		@mkdir -p "$(dir $(IT_KUBECONFIG))"
		kind get kubeconfig --name "$(IT_CLUSTER)" > "$(IT_KUBECONFIG)"
		docker image inspect "$(IT_APP_IMAGE)" >/dev/null 2>&1 || docker pull "$(IT_APP_IMAGE)"
		kind load docker-image "$(IT_APP_IMAGE)" --name "$(IT_CLUSTER)"

it-clean: ## deletes the kind cluster of the integration tests
		kind delete cluster --name "$(IT_CLUSTER)"
		rm -f "$(IT_KUBECONFIG)"

it: it-cluster ## runs the integration tests against the kind cluster
		KFORWARD_IT_KUBECONFIG="$(IT_KUBECONFIG)" KFORWARD_IT_IMAGE="$(IT_APP_IMAGE)" $(GOCMD) test -tags integration -race -count=1 -timeout 5m $(TEST_PKGS)

## All
all: check-quality build test ## runs quality checks, build and tests

## Help
help: ## Show this help
		@echo ''
		@echo 'Usage:'
		@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
		@echo ''
		@echo 'Targets:'
		@awk 'BEGIN {FS = ":.*?## "} { \
				if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "   ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
				else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1, 4)} \
		}' $(MAKEFILE_LIST)
