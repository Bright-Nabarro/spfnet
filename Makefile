.PHONY: all build clean run test proto help build-spf-route build-control build-example run-control run-example

# 变量定义
APP_NAME=spfnet
SPF_ROUTE_BIN=spf_route
CONTROL_BIN=control
EXAMPLE_BIN=simple_sender
BUILD_DIR=bin
PROTO_DIR=proto

# Go 相关
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

# 默认目标
all: build

# 编译所有程序
build: build-spf-route build-control build-example

# 编译 SPF 路由节点
build-spf-route:
	@echo "Building SPF route node..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(SPF_ROUTE_BIN) ./cmd/spf_route

# 编译控制客户端
build-control:
	@echo "Building control client..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(CONTROL_BIN) ./cmd/control

# 编译示例程序
build-example:
	@echo "Building example sender..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(EXAMPLE_BIN) ./examples/simple_sender

# 运行 SPF 路由节点
run: build-spf-route
	@echo "Running SPF route node..."
	./$(BUILD_DIR)/$(SPF_ROUTE_BIN)

# 运行控制客户端
run-control: build-control
	@echo "Running control client..."
	./$(BUILD_DIR)/$(CONTROL_BIN)

# 运行示例程序
run-example: build-example
	@echo "Running example sender..."
	./$(BUILD_DIR)/$(EXAMPLE_BIN)

# 生成 protobuf 代码
proto:
	@echo "Generating protobuf code..."
	cd $(PROTO_DIR) && bash generate.sh

# 运行测试
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# 运行测试并显示覆盖率
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# 清理构建文件
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# 安装依赖
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# 格式化代码
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# 帮助信息
help:
	@echo "SPFNet Makefile Commands:"
	@echo "  make build          - Build all programs (route, control, example)"
	@echo "  make build-spf-route - Build SPF route node"
	@echo "  make build-control  - Build control client"
	@echo "  make build-example  - Build example sender"
	@echo "  make run            - Build and run SPF route node"
	@echo "  make run-control    - Build and run control client"
	@echo "  make run-example    - Build and run example sender"
	@echo "  make proto          - Generate protobuf code"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage"
	@echo "  make clean          - Clean build files"
	@echo "  make deps           - Install dependencies"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Run linter"
	@echo "  make help           - Show this help message"
