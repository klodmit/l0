APP=l0

.PHONY: all tidy build run test docker

all: tidy build

tidy:
	go mod tidy

build:
	go build -o bin/$(APP) ./cmd/l0

run:
	go run ./cmd/l0

test:
	go test ./...

docker:
	docker build -f build/package/Dockerfile -t $(APP):local .
