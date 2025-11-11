.PHONY: build run test clean docker-build migrate

build:
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t agent-svc:latest .

migrate:
	@echo "Migrations are run automatically on startup"

