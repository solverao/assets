BINARY := asset
PKG := ./...

.PHONY: build run test vet lint cover fmt clean

build:
	go build -o $(BINARY) .

run:
	go run . $(filter-out $@,$(MAKECMDGOALS))

test:
	go test $(PKG)

vet:
	go vet $(PKG)

lint:
	golangci-lint run

cover:
	go test -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out

fmt:
	gofmt -w cmd main.go

clean:
	rm -f $(BINARY) coverage.out coverage.html
