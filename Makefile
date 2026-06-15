.PHONY: build up down logs test clean

build:
	docker compose build

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

test:
	docker compose run --rm api go test ./...

clean:
	docker compose down -v
