DOMAIN ?= localhost
CADDY_EMAIL ?= admin@example.com

.PHONY: setup up down logs test build clean

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

clean: down
	docker compose down -v
