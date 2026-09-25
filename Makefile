GOCMD=go
LINTCMD=golangci-lint
GOLANGCI_VERSION?=v2.14.0
GOLANGCI_MIN_VERSION?=2.13.0
GOVET=$(GOCMD) vet
BINARY_NAME=kforward
VERSION?=0.0.1

GREEN		:= $(shell tput -Txterm setaf 2)
YELLOW	:= $(shell tput -Txterm setaf 3)
WHITE		:= $(shell tput -Txterm setaf 7)
CYAN 		:= $(shell tput -Txterm setaf 6)
RESET		:= $(shell tput -Txterm sgr0)

## Tools
# N'installe que si la commande manque ou est trop ancienne : une version
# installée autrement (brew…) est conservée
install-tools: ## installs golangci-lint if missing or older than GOLANGCI_MIN_VERSION
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

## Build
clean: ## cleans binary and other generated files
		$(GOCMD) clean
		rm -rf out/
		rm -f coverage*.out

build: ## Build binary
		$(GOCMD) build -o out/$(BINARY_NAME) .

## Test
test: ## runs tests and generates coverage report
		$(GOCMD) test -race ./internal/... -coverprofile=coverage.out

coverage: test ## displays test coverage report in html code
		$(GOCMD) tool cover -html=coverage.out

.PHONY: install-tools check-quality lint lint-fix lint-config vet fmt fmt-check tidy tidy-check \
	clean build test coverage all help

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
