.PHONY: build test clean

build:
	go build -o hrs .

test:
	go test -v -race ./...

clean:
	rm -f hrs *.db
