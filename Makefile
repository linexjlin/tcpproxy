# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
UPX=upx
BINARY_NAME=tcpp

all: test build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v

release:
	$(GOBUILD) -ldflags="-s -w" -o $(BINARY_NAME) -v
	$(UPX) $(BINARY_NAME)
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
