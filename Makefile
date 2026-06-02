BINARY  = cliplink
LDFLAGS = -ldflags="-s -w"

.PHONY: build build-windows build-all run tidy

build:
	go build $(LDFLAGS) -o $(BINARY) .

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe .

build-all:
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe .

run:
	go run .

tidy:
	go mod tidy
