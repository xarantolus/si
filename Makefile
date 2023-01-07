install:
	go install

build:
	go build

clean:
	go clean
	rm -f *.gif

.PHONY: build install clean
