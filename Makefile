.PHONY: build up down logs generate test clean

build:
	docker compose build

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

generate:
	docker run --rm -v "$(PWD):/workspace" -w /workspace sqlc/sqlc generate

test:
	docker compose run --rm api go test ./...

clean:
	docker compose down -v
