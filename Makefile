.PHONY: bootstrap migrate build test vet run-server run-collector clean

bootstrap:
	./scripts/bootstrap-local-postgres.sh

migrate:
	go run ./cmd/migrate

build:
	mkdir -p bin
	go build -o bin/admin-server ./cmd/admin-server
	go build -o bin/diagnostics-collector ./cmd/collector
	go build -o bin/admin-migrate ./cmd/migrate

test:
	go test ./...

vet:
	go vet ./...

run-server:
	go run ./cmd/admin-server

run-collector:
	go run ./cmd/collector

clean:
	go clean
	rm -f bin/admin-server bin/diagnostics-collector bin/admin-migrate
