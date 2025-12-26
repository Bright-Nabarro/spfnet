# SDK 快速开始指南

本指南帮助业务应用快速集成 SPFNet 分布式数据传输框架。

## 一、为什么使用 SPFNet SDK？

SPFNet SDK 为业务应用提供了简洁的 API，**屏蔽了底层的网络复杂性**：

- ✅ **自动路由**：无需手动指定路径，框架自动计算最短路径
- ✅ **多跳转发**：支持跨多个节点传输数据
- ✅ **动态感知**：自动适应网络拓扑变化
- ✅ **简单易用**：只需 3 行代码即可发送数据

## 二、三步集成

### 1. 导入 SDK

```go
import "spfnet/client"
```

### 2. 创建并启动客户端

```go
c, err := client.NewClient(client.Config{
    NodeID:   "myApp",      // 业务节点 ID
    NodeIP:   "127.0.0.1",  // 节点 IP
    GRPCPort: 5010,         // gRPC 端口
    SerfPort: 7010,         // Serf 端口
    JoinAddr: "127.0.0.1:7001", // 加入已有集群
})
if err != nil {
    log.Fatal(err)
}

c.Start()
defer c.Stop()
```

### 3. 发送数据

```go
err = c.Send("targetNode", []byte("Hello World!"))
```

**就这么简单！** 框架会自动：
1. 查询路由表找到最短路径
2. 沿最优路径多跳转发
3. 确保数据到达目标节点

## 三、完整示例

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "spfnet/client"
)

func main() {
    // 创建客户端
    c, err := client.NewClient(client.Config{
        NodeID:   "businessApp1",
        NodeIP:   "127.0.0.1",
        GRPCPort: 5020,
        SerfPort: 7020,
        JoinAddr: "127.0.0.1:7001",
    })
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    // 启动客户端
    if err := c.Start(); err != nil {
        log.Fatalf("Failed to start: %v", err)
    }
    defer c.Stop()

    log.Println("Client started, waiting for topology sync...")
    time.Sleep(2 * time.Second)

    // 发送数据到目标节点
    if err := c.Send("nodeA", []byte("Hello from business layer!")); err != nil {
        log.Printf("Send failed: %v", err)
    } else {
        log.Println("Data sent successfully!")
    }

    // 持续运行
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
}
```

## 四、运行示例

### 前提条件

先启动至少一个路由节点：

```bash
# 终端 1：启动路由节点 A
bin/spf_route -node nodeA

# 终端 2：启动路由节点 B
bin/spf_route -node nodeB
```

### 运行业务应用

```bash
# 编译示例
make build-example

# 运行示例
bin/simple_sender
```

或者：

```bash
make run-example
```

## 五、API 参考

### `NewClient(cfg Config) (*Client, error)`

创建客户端实例。

**配置参数：**
- `NodeID` - 节点唯一标识（必需）
- `NodeIP` - 节点 IP 地址（必需）
- `GRPCPort` - gRPC 服务端口（必需）
- `SerfPort` - Serf 集群端口（必需）
- `JoinAddr` - 要加入的集群地址（可选）
- `AppConfigPath` - 配置文件路径（可选）

### `Start() error`

启动客户端。自动完成：
- 加入 Serf 集群
- 同步拓扑信息
- 计算路由表
- 启动 gRPC 服务

### `Send(destination string, data []byte) error`

发送数据到目标节点。

**参数：**
- `destination` - 目标节点 ID
- `data` - 要发送的字节数据

框架自动处理路由查询和多跳转发。

### `AddLink(neighborID, neighborAddr string, cost float64) error`

手动添加邻居链路。

**参数：**
- `neighborID` - 邻居节点 ID
- `neighborAddr` - 邻居地址（格式：`ip:port`）
- `cost` - 链路成本（0 表示自动探测）

### `Stop()`

停止客户端，释放资源。

## 六、使用场景

### 场景 1：分布式数据库节点间通信

```go
// 数据库节点启动时
dbClient, _ := client.NewClient(client.Config{
    NodeID: "db-node-1",
    // ... 其他配置
})
dbClient.Start()

// 发送数据同步请求到其他数据库节点
dbClient.Send("db-node-2", serializeData(record))
```

### 场景 2：微服务间消息传递

```go
// 订单服务向库存服务发送消息
orderService.Send("inventory-service", orderMessage)
```

### 场景 3：广域网数据采集

```go
// 边缘节点向中心节点上报数据
edgeNode.Send("center-node", sensorData)
```

## 七、注意事项

1. **等待拓扑同步**：启动后建议等待 1-2 秒让拓扑信息同步完成
2. **端口冲突**：确保 GRPCPort 和 SerfPort 不与其他进程冲突
3. **集群地址**：首个节点无需 JoinAddr，后续节点需指定已有节点的 Serf 地址

## 八、故障排查

### 问题：发送失败，提示 "no route to destination"

**原因**：目标节点不在拓扑中，或拓扑尚未同步完成。

**解决**：
- 检查目标节点是否已启动
- 等待更长时间让拓扑同步
- 手动添加链路：`c.AddLink("targetNode", "ip:port", 0)`

### 问题：无法加入集群

**原因**：JoinAddr 配置错误或目标节点未启动。

**解决**：
- 检查 JoinAddr 格式是否正确（`ip:serf_port`）
- 确认目标节点的 Serf 服务正在运行

## 九、更多示例

查看 `examples/simple_sender/main.go` 获取更多用法示例。
