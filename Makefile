.PHONY: build run clean test

build:
	go build -o kconduit cmd/kconduit/main.go

run: build
	./kconduit

clean:
	rm -f kconduit

test:
	go test ./...