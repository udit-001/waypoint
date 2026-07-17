BIN           = bin/waypoint
CMD           = ./cmd/waypoint
MODULE        = github.com/udit-001/waypoint

# Colors for output
BLUE  = \033[36m
RESET = \033[0m

.PHONY: all build install dev frontend clean distclean fmt test-frontend check

all: frontend build

## Build the full binary (frontend + Go)
build: frontend
	@echo "$(BLUE)→ Building $(BIN)...$(RESET)"
	@mkdir -p bin
	CGO_ENABLED=0 go build -o $(BIN) $(CMD)

## Install via Go (compiles from source)
install:
	@echo "$(BLUE)→ Installing $(BIN) via go install...$(RESET)"
	go install $(CMD)@latest

## Frontend: install deps + build
frontend:
	@echo "$(BLUE)→ Building frontend...$(RESET)"
	cd web && pnpm install --frozen-lockfile && pnpm build

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
	go test ./...

## Run frontend tests
test-frontend:
	@echo "$(BLUE)→ Running frontend tests...$(RESET)"
	cd web && pnpm test

## Pre-commit gate: formatting check, vet, all tests
check:
	@echo "$(BLUE)→ Pre-commit gate...$(RESET)"
	@test -z "$$(gofmt -l .)" || { echo "  gofmt needed — run 'make fmt'"; exit 1; }
	go vet ./...
	go test ./...
	cd web && pnpm test
