# Run from a POSIX shell (MSYS2, Git Bash, Linux, macOS).

BINARY  := etmapping-setup
PKG     := ./cmd/etmapping-setup
DIST    := dist
VERSION ?= dev

GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all windows linux macos run vet clean

all: windows linux macos

windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(DIST)/$(BINARY)-windows.exe $(PKG)

linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(DIST)/$(BINARY)-linux $(PKG)

macos:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(DIST)/$(BINARY)-macos-arm64 $(PKG)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(DIST)/$(BINARY)-macos-intel $(PKG)

run:
	go run $(PKG)

vet:
	go vet ./...

clean:
	rm -rf $(DIST)
