.PHONY: help build up down start logs clean reset test status health init

help:
	@echo "Piano PIR + Plinko PoC - Makefile Commands"
	@echo ""
	@echo "Setup:"
	@echo "  make init        - Initialize environment (.env file)"
	@echo "  make build       - Build all Docker services"
	@echo "  make start       - Start all services (background)"
	@echo "  make up          - Start all services (foreground)"
	@echo ""
	@echo "Development:"
	@echo "  make logs        - View logs from all services"
	@echo "  make status      - Show service status"
	@echo "  make health      - Check service health endpoints"
	@echo ""
	@echo "Management:"
	@echo "  make stop        - Stop all services"
	@echo "  make restart     - Restart all services"
	@echo "  make clean       - Stop and remove volumes"
	@echo "  make reset       - Full reset and rebuild"
	@echo ""
	@echo "Testing:"
	@echo "  make test               - Run full test suite"
	@echo "  make test-privacy       - Privacy verification only"
	@echo "  make test-performance   - Performance tests only"
	@echo "  make test-addressing    - Service addressing configuration"
	@echo ""

init:
	@echo "Initializing PoC environment..."
	@./scripts/init-poc.sh

build:
	@echo "Building all services..."
	docker-compose build

start:
	@echo "Starting services in background..."
	docker-compose up -d
	@echo ""
	@echo "✅ Services started! Access wallet at: http://localhost:5173"
	@echo "   Run 'make logs' to view logs or 'make status' to check service status"

up:
	@echo "Starting services in foreground..."
	@echo "(Press Ctrl+C to stop)"
	@echo ""
	docker-compose up

stop:
	@echo "Stopping services..."
	docker-compose down

logs:
	@echo "Following logs (Ctrl+C to stop)..."
	docker-compose logs -f

status:
	@echo "Service status:"
	@docker-compose ps

health:
	@echo "Checking service health..."
	@echo ""
	@echo "PIR Server:"
	@curl -s http://localhost:3000/health | jq . || echo "  ❌ Not reachable"
	@echo ""
	@echo "Plinko Update Service:"
	@curl -s http://localhost:3001/health | jq . || echo "  ❌ Not reachable"
	@echo ""
	@echo "CDN Mock:"
	@curl -s -I http://localhost:8080/hint.bin | head -1 || echo "  ❌ Not reachable"
	@echo ""
	@echo "Ethereum Mock:"
	@curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' http://localhost:8545 | jq . || echo "  ❌ Not reachable"

restart:
	@echo "Restarting services..."
	docker-compose restart

clean:
	@echo "Stopping services and removing volumes..."
	docker-compose down -v

reset:
	@echo "Full reset: cleaning and rebuilding..."
	@./scripts/reset.sh

test:
	@echo "Running full test suite..."
	@echo ""
	@echo "======================================"
	@echo "PRIVACY VERIFICATION TESTS"
	@echo "======================================"
	@./scripts/test-privacy.sh
	@echo ""
	@echo "======================================"
	@echo "PERFORMANCE VALIDATION TESTS"
	@echo "======================================"
	@./scripts/test-performance.sh

test-privacy:
	@echo "Running privacy verification tests..."
	@./scripts/test-privacy.sh

test-performance:
	@echo "Running performance validation tests..."
	@./scripts/test-performance.sh

test-addressing:
	@echo "Testing service addressing configuration..."
	@./scripts/test-addressing.sh

.DEFAULT_GOAL := help
