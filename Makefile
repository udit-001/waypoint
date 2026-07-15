BIN           = waypoint
CMD           = ./cmd/waypoint
MODULE        = github.com/udit-001/waypoint

# Colors for output
BLUE  = \033[36m
RESET = \033[0m

.PHONY: all build install dev frontend clean distclean

all: frontend build

## Build the full binary (frontend + Go)
build: frontend
	@echo "$(BLUE)→ Building $(BIN)...$(RESET)"
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

## Clean build artifacts
clean:
	@echo "$(BLUE)→ Cleaning...$(RESET)"
	rm -f $(BIN)
	go clean

## Remove frontend build output (stub so go build still works)
distclean: clean
	@echo "$(BLUE)→ Removing frontend dist...$(RESET)"
	rm -rf web/dist
	mkdir -p web/dist
	echo "Frontend not built — run 'make frontend' first" > web/dist/index.html

## Run tests
test:
	@echo "$(BLUE)→ Running tests...$(RESET)"
	go test ./...
