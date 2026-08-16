BIN           = bin/waypoint
CMD           = ./cmd/waypoint
MODULE        = github.com/udit-001/waypoint

# Colors for output (real escape bytes via printf — echo won't interpret \033)
BLUE  = $(shell printf '\033[36m')
RESET = $(shell printf '\033[0m')

.PHONY: all build install dev start stop frontend clean distclean fmt test test-race test-frontend check install-hooks

all: frontend build

## Build the full binary (frontend + Go)
build: frontend
	@echo "$(BLUE)→ Building $(BIN)...$(RESET)"
	@mkdir -p bin
	CGO_ENABLED=0 go build -o $(BIN) $(CMD)

## Install into GOBIN (compiles from source)
install:
	@printf "$(BLUE)→ Installing waypoint into %s...$(RESET)\n" "$${GOBIN:-$$(go env GOPATH)/bin}"
	@CGO_ENABLED=0 go install $(CMD)

## Frontend: install deps + build (quiet — vite logs only on failure)
frontend:
	@echo "$(BLUE)→ Building frontend...$(RESET)"
	@cd web && pnpm install --frozen-lockfile --silent && pnpm --silent run build --logLevel error

## Start the web UI in the background (daemon). Stop with `make stop`.
start: frontend
	@echo "$(BLUE)→ Starting waypoint in background...$(RESET)"
	@echo "$(BLUE)→ Use 'make stop' to stop it$(RESET)"
	@CGO_ENABLED=0 go run $(CMD) start

## Stop the background server.
stop:
	@echo "$(BLUE)→ Stopping waypoint...$(RESET)"
	@CGO_ENABLED=0 go run $(CMD) stop

## Dev: run the backend with live frontend proxy
dev:
	@echo "$(BLUE)→ Starting backend (frontend is served by Vite dev server)...$(RESET)"
	@echo "$(BLUE)→ Start the Vite dev server in another terminal: cd web && pnpm dev$(RESET)"
	CGO_ENABLED=0 go run $(CMD) start --foreground

## Tidy Go modules
tidy:
	@echo "$(BLUE)→ Tidying Go modules...$(RESET)"
	go mod tidy

## Format Go code
fmt:
	@echo "$(BLUE)→ Formatting Go...$(RESET)"
	gofmt -s -w .

## Clean build artifacts
clean:
	@echo "$(BLUE)→ Cleaning...$(RESET)"
	rm -rf bin
	go clean

## Remove frontend build output (stub so go build still works)
distclean: clean
	@echo "$(BLUE)→ Removing frontend dist...$(RESET)"
	rm -rf web/dist
	mkdir -p web/dist
	echo "Frontend not built — run 'make frontend' first" > web/dist/index.html

## Run Go tests
test:
	@echo "$(BLUE)→ Running Go tests...$(RESET)"
	CGO_ENABLED=0 go test ./...

## Run Go tests with the race detector (CI parity; slower than 'test')
test-race:
	@echo "$(BLUE)→ Running Go tests with -race...$(RESET)"
	CGO_ENABLED=0 go test -race ./...

## Run frontend tests
test-frontend:
	@echo "$(BLUE)→ Running frontend tests...$(RESET)"
	cd web && pnpm test

## Pre-commit gate: formatting check, vet, all tests
check:
	@echo "$(BLUE)→ Pre-commit gate...$(RESET)"
	@test -z "$$(gofmt -l .)" || { echo "  gofmt needed — run 'make fmt'"; exit 1; }
	go vet ./...
	CGO_ENABLED=0 go test ./...
	cd web && pnpm test

## Install git hooks (gofmt check on staged .go files)
install-hooks:
	@echo "$(BLUE)→ Configuring .githooks as hooks path...$(RESET)"
	git config core.hooksPath .githooks
