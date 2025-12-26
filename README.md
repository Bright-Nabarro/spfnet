# SPFNet: 分布式数据传输框架

## 项目概述

本项目实现了一个基于 **SPF（Shortest Path First）算法** 的分布式数据传输框架，专为广域网（WAN）环境下的分布式应用系统设计。

### 解决的问题

在分布式系统规模扩大、业务量增长时，节点间的数据传输会变得频繁，容易导致某些节点性能受限。本框架通过**两层架构**解决这一问题：

1. **路由层**：自动收集网络拓扑，使用 SPF 算法计算最优路由，维护路由表
2. **数据传输层**：为业务应用提供简洁的 API，屏蔽复杂的网络环境，自动按最短路径转发数据

### 核心特性

- **自动路由计算**：基于 SPF 算法动态计算最短路径
- **多跳转发**：支持跨多个节点的数据包自动转发
- **拓扑自适应**：基于 Serf 实现节点发现和拓扑同步
- **链路质量感知**：实时探测 RTT，动态调整路由
- **简洁 SDK**：业务层只需 3 行代码即可发送数据，无需关心底层路由

### 系统架构

```
┌─────────────────────────────────────────┐
│         业务应用层（Business App）        │
│     使用 SDK 发送/接收数据               │
└──────────────┬──────────────────────────┘
               │ SPFNet SDK
┌──────────────▼──────────────────────────┐
│         数据传输层（Forward Layer）      │
│  • 数据包转发                            │
│  • 路由查询                              │
│  • gRPC 通信                             │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│          路由层（Route Layer）           │
│  • SPF 算法计算最短路径                  │
│  • 维护路由表                            │
│  • 拓扑管理                              │
│  • Serf 集群管理                         │
└─────────────────────────────────────────┘
```

## 快速开始（业务应用集成）

### 使用 SDK（推荐）

```go
import "spfnet/client"

// 1. 创建客户端
c, _ := client.NewClient(client.Config{
    NodeID:   "myApp",
    NodeIP:   "127.0.0.1",
    GRPCPort: 5010,
    SerfPort: 7010,
    JoinAddr: "127.0.0.1:7001", // 加入已有集群
})

// 2. 启动（自动加入集群、同步拓扑、计算路由）
c.Start()
defer c.Stop()

// 3. 发送数据（自动沿最短路径转发）
c.Send("targetNode", []byte("Hello World!"))
```

**详细文档**：[SDK 快速开始指南](docs/SDK_QUICKSTART.md)

## 路由节点使用方法

### 启动节点

**方式一：使用配置文件**
```bash
bin/spf_route -node nodeA
```

**方式二：命令行参数**
```bash
bin/spf_route -id node1 -ip 127.0.0.1 -port 5001 -serf-port 7001 -join 127.0.0.1:7002
```

### 命令行参数说明
- `-app-config`: 应用配置文件路径（默认：configs/app.toml）
- `-config`: 拓扑配置文件路径（默认：configs/topology.toml）
- `-node`: 配置文件中的节点名称
- `-id`: 节点 ID
- `-ip`: 节点 IP
- `-port`: gRPC 端口
- `-serf-port`: Serf 端口
- `-join`: 要加入的节点地址

### 控制命令

`bin/control` 是控制节点的命令行工具，支持以下命令：

#### 1. ping - 测试节点连通性
```bash
bin/control -server localhost:5001 -cmd ping
```

#### 2. addlink - 添加邻居链路
```bash
bin/control -server localhost:5001 -cmd addlink \
  -neighbor nodeB \
  -neighbor-addr localhost:5002 \
  -cost 10.0 \
  -auto-probe true
```

**参数说明：**
- `-neighbor`: 邻居节点 ID（必需）
- `-neighbor-addr`: 邻居节点 gRPC 地址，格式 ip:port（必需）
- `-cost`: 链路成本，0 表示自动探测（默认：0）
- `-auto-probe`: 是否自动探测链路质量（默认：true）

#### 3. sendpacket - 发送数据包
```bash
bin/control -server localhost:5001 -cmd sendpacket \
  -source-addr localhost:5001 \
  -source nodeA \
  -dest nodeC \
  -payload "hello world" \
  -packet-id "pkt-001"
```

**参数说明：**
- `-source-addr`: 源节点 gRPC 地址，格式 ip:port（必需）
- `-source`: 源节点 ID（必需）
- `-dest`: 目标节点 ID（必需）
- `-payload`: 数据包负载内容（默认："hello"）
- `-packet-id`: 数据包 ID，留空则自动生成（可选）

#### 通用参数
- `-server`: 目标节点地址，格式 ip:port（默认：localhost:5001）
- `-cmd`: 要执行的命令（必需）：ping, addlink, sendpacket

## SDK 使用（业务应用集成）

### 快速开始

业务应用可以通过导入 `spfnet` 包快速集成分布式数据传输功能，无需关心底层路由和网络细节。

```go
package main

import (
    "log"
    "spfnet/spfnet"
)

func main() {
    // 1. 创建应用节点
    node, err := spfnet.NewNode(spfnet.Config{
        NodeID:   "myApp",
        NodeIP:   "127.0.0.1",
        GRPCPort: 5003,
        SerfPort: 7003,
        JoinAddr: "127.0.0.1:7001", // 加入已有集群
    })
    if err != nil {
        log.Fatalf("Failed to create node: %v", err)
    }

    // 2. 启动节点（自动加入集群、同步拓扑、计算路由）
    if err := node.Start(); err != nil {
        log.Fatalf("Failed to start node: %v", err)
    }
    defer node.Stop()

    // 3. 发送数据到目标节点（自动沿最短路径传输）
    if err := node.Send("nodeA", []byte("Hello World!")); err != nil {
        log.Printf("Failed to send: %v", err)
    }
}
```

### SDK API 说明

#### `NewNode(cfg Config) (*Node, error)`
创建应用节点实例

**配置参数：**
- `NodeID`: 节点 ID（必需）
- `NodeIP`: 节点 IP 地址（必需）
- `GRPCPort`: gRPC 服务端口（必需）
- `SerfPort`: Serf 集群端口（必需）
- `JoinAddr`: 加入的集群地址，如 "127.0.0.1:7001"（可选）
- `AppConfigPath`: 应用配置文件路径（可选，默认 "configs/app.toml"）

#### `Start() error`
启动应用节点，自动完成：
- 加入集群
- 同步拓扑信息
- 计算最短路径
- 启动 gRPC 服务

#### `Send(destination string, data []byte) error`
发送数据到目标节点，框架自动：
- 查询路由表
- 选择最短路径
- 多跳转发至目的地

**参数：**
- `destination`: 目标节点 ID
- `data`: 要发送的数据

#### `AddLink(neighborID, neighborAddr string, cost float64) error`
添加到邻居节点的链路

**参数：**
- `neighborID`: 邻居节点 ID
- `neighborAddr`: 邻居节点地址，格式 "ip:port"
- `cost`: 链路成本，0 表示自动探测

#### `Stop()`
停止应用节点

### 完整示例

参考 [examples/simple_sender/main.go](examples/simple_sender/main.go) 和 [examples/sdk_usage/main.go](examples/sdk_usage/main.go) 查看完整的业务应用示例。

运行示例：
```bash
# 先启动两个路由节点
bin/spf_route -node nodeA &
bin/spf_route -node nodeB &

# 运行业务应用
go run examples/simple_sender/main.go
```

## TODO
- [x] gRPC 数据包转发
- [x] gRPC 链路更新
- [x] 拓扑定时全量同步
- [x] 应用层 SDK 接口
- [ ] 支持根据 IP 地址匹配节点