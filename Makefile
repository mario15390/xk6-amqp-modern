MAKEFLAGS += --silent

all: clean format test build

## help: Prints a list of available build targets.
help:
	echo "Usage: make <OPTIONS> ... <TARGETS>"
	echo ""
	echo "Available targets are:"
	echo ''
	sed -n 's/^##//p' ${PWD}/Makefile | column -t -s ':' | sed -e 's/^/ /'
	echo
	echo "Targets run by default are: `sed -n 's/^all: //p' ./Makefile | sed -e 's/ /, /g' | sed -e 's/\(.*\), /\1, and /'`"

## clean: Removes any previously created build artifacts.
clean:
	rm -f ./k6

## build: Builds a custom 'k6' with the local extension.
build:
	go install go.k6.io/xk6/cmd/xk6@latest
	xk6 build --with $(shell go list -m)=.

## format: Applies Go formatting to code.
format:
	go fmt ./...

## test: Executes unit tests with coverage.
test:
	go test -v -race -cover -count=1 ./...

## docker-build: Build k6 binary with extension inside Docker.
docker-build:
	docker compose build k6-build

## docker-unit-test: Run unit tests in Docker.
docker-unit-test:
	docker compose run --rm unit-tests

## docker-integration-test: Run integration tests in Docker (starts RabbitMQ).
docker-integration-test:
	docker compose run --rm integration-tests

## docker-k6-test: Run k6 integration script in Docker.
docker-k6-test:
	docker compose run --rm k6-integration

## docker-test-all: Run all tests in Docker (unit + integration + k6).
docker-test-all: docker-unit-test docker-integration-test docker-k6-test

## docker-performance: Run k6 performance test (manual, with RabbitMQ).
docker-performance:
	docker compose run --rm k6-performance

## docker-clean: Stop and remove all Docker containers.
docker-clean:
	docker compose down -v --remove-orphans

.PHONY: build clean format help test \
        docker-build docker-unit-test docker-integration-test \
        docker-k6-test docker-test-all docker-performance docker-clean
