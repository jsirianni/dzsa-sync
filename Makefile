.PHONY: lint secure test build tidy

lint:
	go tool revive ./...

secure:
	go tool gosec -exclude=G704 ./...

test:
	go test -race ./...

build:
	go build -o dzsa-sync ./cmd/dzsasync

tidy:
	go mod tidy
