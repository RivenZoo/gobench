.PHONY: build

build:
	go build -o build/gobench *.go

clean:
	rm -f build/gobench