# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
VERSIONINFO=".git/logs"
BINDCMD="go-bindata"
UPX=upx
BINARY_NAME=tcpp

all: test build
build:
	$(BINDCMD) $(VERSIONINFO)
	$(GOBUILD) -o $(BINARY_NAME) -v

release:
	$(BINDCMD) $(VERSIONINFO)
	$(GOBUILD) -ldflags="-s -w" -o $(BINARY_NAME) -v
	$(UPX) $(BINARY_NAME)
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
