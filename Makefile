.PHONY: bootstrap migrate web-install web-test web-build build test vet run-server run-collector clean

bootstrap:
	./scripts/bootstrap-local-postgres.sh

migrate:
	go run ./cmd/migrate

web-install:
	cd web && npm ci

web-test:
	cd web && npm run lint && npm run typecheck && npm test

web-build:
	cd web && npm run build

build: web-build
	mkdir -p bin
	go build -o bin/admin-server ./cmd/admin-server
	go build -o bin/diagnostics-collector ./cmd/collector
	go build -o bin/admin-migrate ./cmd/migrate

test: web-test web-build
	go test ./cmd/... ./internal/...

vet:
	go vet ./cmd/... ./internal/...

run-server:
	go run ./cmd/admin-server

run-collector:
	go run ./cmd/collector

clean:
	go clean
	rm -f bin/admin-server bin/diagnostics-collector bin/admin-migrate
	rm -rf internal/server/web-dist
