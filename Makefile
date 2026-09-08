.PHONY: build test clean gui

build:
	go build -o hrs .

test:
	go test -v -race ./...

gui:
	cd gui-macos && swift run HrsGUI

clean:
	rm -f hrs *.db
