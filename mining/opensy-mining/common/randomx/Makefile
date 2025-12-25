# RandomX CGO Build

RANDOMX_VERSION := v1.1.10
RANDOMX_DIR := randomx-src
LIB_DIR := lib
INCLUDE_DIR := include

.PHONY: all randomx clean test

all: randomx

randomx: $(LIB_DIR)/librandomx.a

$(LIB_DIR)/librandomx.a: $(RANDOMX_DIR)
	@echo "Building RandomX static library..."
	@mkdir -p $(LIB_DIR) $(INCLUDE_DIR)
	cd $(RANDOMX_DIR) && cmake -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF .
	cd $(RANDOMX_DIR) && $(MAKE) -j$$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)
	cp $(RANDOMX_DIR)/librandomx.a $(LIB_DIR)/
	cp $(RANDOMX_DIR)/src/randomx.h $(INCLUDE_DIR)/
	@echo "RandomX library built successfully!"
	@echo "  Library: $(LIB_DIR)/librandomx.a"
	@echo "  Header:  $(INCLUDE_DIR)/randomx.h"

$(RANDOMX_DIR):
	@echo "Cloning RandomX $(RANDOMX_VERSION)..."
	git clone --depth 1 --branch $(RANDOMX_VERSION) https://github.com/tevador/RandomX.git $(RANDOMX_DIR)

clean:
	rm -rf $(RANDOMX_DIR) $(LIB_DIR) $(INCLUDE_DIR)

test:
	CGO_ENABLED=1 go test -v ./...

bench:
	CGO_ENABLED=1 go test -bench=. -benchmem ./...

# Platform-specific builds
.PHONY: linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o bin/linux-amd64/ ./...

linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build -o bin/linux-arm64/ ./...

darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -o bin/darwin-amd64/ ./...

darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o bin/darwin-arm64/ ./...
