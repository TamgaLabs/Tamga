DOMAIN ?= localhost

.PHONY: setup up down logs test build clean smoke-test frontend-dev frontend-build sdlc-prepare sdlc-smoke sdlc-teardown

-include .env
export

setup:
	@test -f .env || cp .env.example .env
	@echo "Edit .env and run: make up"

build:
	docker compose build

up:
	@test -f .env || cp .env.example .env
	docker compose build agent egress-proxy
	docker compose up -d
	@echo ""
	@echo "Frontend: https://$(DOMAIN)"; \
	echo "API:      https://$(DOMAIN)/api"

down:
	docker compose down

logs:
	docker compose logs -f backend

test:
	go test ./backend/...

smoke-test:
	@./scripts/smoke-test.sh

# Local, visual-only frontend preview. It does not start Docker or call Tamga APIs.
frontend-dev:
	@cd frontend && env -u PORT npm run dev:offline -- --port 3000

frontend-build:
	@cd frontend && env -u PORT npm run build:offline

sdlc-prepare:
	@test -n "$(MANIFEST)" || (echo "Usage: make sdlc-prepare MANIFEST=/tmp/tamga-sdlc-<task>.manifest" >&2; exit 2)
	@./scripts/sdlc-environment.sh prepare "$(MANIFEST)" "$(CURDIR)"

sdlc-smoke:
	@test -n "$(MANIFEST)" || (echo "Usage: make sdlc-smoke MANIFEST=/tmp/tamga-sdlc-<task>.manifest" >&2; exit 2)
	@./scripts/sdlc-environment.sh smoke "$(MANIFEST)" "$(CURDIR)"

sdlc-teardown:
	@test -n "$(MANIFEST)" || (echo "Usage: make sdlc-teardown MANIFEST=/tmp/tamga-sdlc-<task>.manifest" >&2; exit 2)
	@./scripts/sdlc-environment.sh cleanup "$(MANIFEST)"

clean: down
	docker compose down -v
