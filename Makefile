# Git-Go Makefile
# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=git-go

# Build targets
.PHONY: all build clean test deps tidy fmt vet run install help

all: deps build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v .

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

deps:
	$(GOMOD) download

tidy:
	$(GOMOD) tidy

fmt:
	$(GOCMD) fmt ./...

vet:
	$(GOCMD) vet ./...

run:
	$(GOBUILD) -o $(BINARY_NAME) -v .
	./$(BINARY_NAME)

install:
	$(GOCMD) install

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME) -v .

build-mac:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME) -v .

build-all: build-linux build-mac

dev: deps fmt vet test build

check: fmt vet test

help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  deps         - Download dependencies"
	@echo "  tidy         - Tidy go modules"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  run          - Build and run the application"
	@echo "  install      - Install the binary"
	@echo "  build-linux  - Cross compile for Linux"
	@echo "  build-mac    - Cross compile for macOS"
	@echo "  build-all    - Cross compile for all platforms"
	@echo "  dev          - Development build (deps + fmt + vet + test + build)"
	@echo "  check        - Run checks (fmt + vet + test)"
	@echo "  help         - Show this help message"
