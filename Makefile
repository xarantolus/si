install:
	go install -ldflags "-s -w"

build:
	go build -ldflags "-s -w"

clean:
	go clean
	rm -f *.gif

.PHONY: install clean build
