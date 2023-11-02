# Variables
BINARY_NAME=mm-packet-pull
GO=go

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY_NAME)_linux_amd64

# Clean Up
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)_linux_amd64
