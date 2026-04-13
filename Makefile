.PHONY: build test clean

build:
	go build -o worklog .

test:
	go test -v -race ./...

clean:
	rm -f worklog worklog.db
