# SPFNet 项目总结

## 课题要求对照

### 要求概述

> 广域网上的数据传输是分布式数据库系统的重要组成部分之一，该系统的设计难点在于当分布式应用系统的规模不断扩大、业务量不断增大的时候，系统中各节点的数据传输将变得极为频繁，这就可能导致某些节点的传输性能受限，从而影响业务。
>
> 本课题要求同学们针对此问题设计并实现了一个基于 SPF（Shortest Path First，最短路径优先）的分布式数据传输框架。该框架由两个层次组成：数据传输层和路由层。数据传输层为分布式应用系统的业务层提供便捷的数据传输接口，使得业务层只需关注业务逻辑，屏蔽了复杂的网络环境。路由层则负责收集网络上的节点信息，并利用 SPF 算法计算出所有最佳路由，将其保存到一张路由表中，以此为数据传输层提供路由服务，保证数据传输沿着最短的路径到达目的节点。

## 实现情况

### ✅ 已完成功能

#### 1. 路由层（Route Layer）

**节点信息收集**
- [x] 基于 Serf 实现节点自动发现
- [x] 实时同步拓扑信息到所有节点
- [x] 维护节点状态（在线/离线）
- [x] 支持节点动态加入/退出

**SPF 算法实现**
- [x] 使用 Dijkstra 算法计算最短路径
- [x] 支持自定义链路成本
- [x] 自动探测 RTT 作为链路成本
- [x] 拓扑变化时自动重新计算路由

**路由表管理**
- [x] 为每个目标节点维护下一跳信息
- [x] 记录到达每个节点的最短距离
- [x] 支持路由表查询
- [x] 路由表自动更新

**文件位置：**
- `internal/route/spf.go` - SPF 算法实现
- `internal/route/route_table.go` - 路由表数据结构
- `internal/route/route_manager.go` - 路由管理器
- `internal/route/topology_sync.go` - 拓扑同步
- `internal/route/node.go` - 节点管理

#### 2. 数据传输层（Forward Layer）

**便捷的数据传输接口**
- [x] 封装 SDK（`client/client.go`）
- [x] 简洁的 API 设计（3 行代码即可发送数据）
- [x] 屏蔽底层网络复杂性
- [x] 业务层无需关心路由和转发细节

**多跳数据包转发**
- [x] 基于路由表的自动转发
- [x] 支持跨多个节点传输
- [x] 记录数据包经过的路径
- [x] gRPC 实现可靠传输

**连接管理**
- [x] gRPC 连接池
- [x] 自动建立和维护连接
- [x] 连接复用

**文件位置：**
- `client/client.go` - 对外 SDK
- `internal/route/forward_manager.go` - 转发管理器
- `internal/route/grpc_server.go` - gRPC 服务实现
- `proto/node.proto` - 通信协议定义

#### 3. 辅助功能

**配置管理**
- [x] 支持配置文件方式
- [x] 支持命令行参数方式
- [x] 灵活的配置加载机制

**控制工具**
- [x] `bin/control` 命令行工具
- [x] 支持添加链路
- [x] 支持发送测试数据包

**构建工具**
- [x] Makefile 自动化构建
- [x] 支持编译路由节点、控制工具、示例程序

## 核心技术点

### 1. SPF 算法（Dijkstra）

```go
// internal/route/spf.go
func (calc *SPFCalculator) ComputeRoutes(sourceNodeID string, topology *Topology) (*RouteTable, error)
```

- 使用优先队列优化，时间复杂度 O((V+E)logV)
- 支持动态拓扑更新
- 计算到所有节点的最短路径和下一跳

### 2. 拓扑同步机制

**两种同步方式：**
1. **增量同步**：链路变化时通过 Serf UserEvent 广播
2. **全量同步**：定期全量同步确保数据一致性

```go
// internal/route/topology_sync.go
func (ts *TopologySync) RegisterLink(from, to string, cost float64) error
func (ts *TopologySync) Start()
```

### 3. 数据包转发流程

```
业务层调用 SDK
    ↓
查询路由表获取下一跳
    ↓
通过 gRPC 发送到下一跳
    ↓
下一跳节点接收后继续转发
    ↓
最终到达目标节点
```

### 4. SDK 设计

**设计原则：简洁易用，屏蔽复杂性**

```go
// 业务应用只需要这样使用：
client, _ := client.NewClient(config)
client.Start()
client.Send("targetNode", data)
```

SDK 自动处理：
- 加入集群
- 拓扑同步
- 路由计算
- 数据转发

## 项目结构

```
spfnet/
├── client/              # SDK 客户端
│   ├── client.go       # 核心 SDK 实现
│   └── example_test.go # 使用示例
├── cmd/
│   ├── spf_route/      # 路由节点程序
│   └── control/        # 控制工具
├── internal/route/      # 核心实现
│   ├── spf.go          # SPF 算法
│   ├── route_table.go  # 路由表
│   ├── route_manager.go # 路由管理器
│   ├── forward_manager.go # 转发管理器
│   ├── topology_sync.go # 拓扑同步
│   ├── grpc_server.go  # gRPC 服务
│   ├── node.go         # 节点管理
│   ├── bootstrap.go    # 启动引导
│   └── config.go       # 配置管理
├── proto/              # protobuf 定义
│   └── node.proto      # 通信协议
├── examples/           # 示例程序
│   └── simple_sender/  # 简单发送示例
├── docs/               # 文档
│   └── SDK_QUICKSTART.md # SDK 快速开始
└── README.md           # 项目说明
```

## 使用示例

### 场景 1：启动三节点集群

```bash
# 终端 1：启动节点 A
bin/spf_route -node nodeA

# 终端 2：启动节点 B
bin/spf_route -node nodeB

# 终端 3：启动节点 C
bin/spf_route -node nodeC
```

### 场景 2：建立链路

```bash
# nodeA 连接到 nodeB
bin/control -server localhost:5001 -cmd addlink \
  -neighbor nodeB -neighbor-addr localhost:5002

# nodeB 连接到 nodeC
bin/control -server localhost:5002 -cmd addlink \
  -neighbor nodeC -neighbor-addr localhost:5003
```

### 场景 3：业务应用发送数据

```go
// 业务应用代码
c, _ := client.NewClient(client.Config{
    NodeID:   "businessApp",
    NodeIP:   "127.0.0.1",
    GRPCPort: 5010,
    SerfPort: 7010,
    JoinAddr: "127.0.0.1:7001",
})
c.Start()

// 从 businessApp -> nodeA -> nodeB -> nodeC
c.Send("nodeC", []byte("Hello!"))
```

数据会自动沿最短路径转发！

## 技术栈

- **语言**：Go 1.21+
- **集群管理**：Hashicorp Serf
- **通信协议**：gRPC + Protocol Buffers
- **算法**：Dijkstra 最短路径算法

## 特色与亮点

### 1. 完整的两层架构

严格按照课题要求实现了路由层和数据传输层的分离：
- 路由层专注于路径计算和拓扑管理
- 数据传输层专注于为业务层提供服务

### 2. 真正的"屏蔽复杂性"

通过 SDK 实现了对业务层的完全透明：
- 业务代码不需要知道网络拓扑
- 不需要手动指定路由
- 不需要处理连接管理
- 只需调用 `Send()` 方法

### 3. 生产级设计

- 连接池管理，提高性能
- 并发安全的数据结构
- 优雅的启动和关闭
- 完善的错误处理
- 详细的日志输出

### 4. 可扩展性

- 支持自定义链路成本算法
- 支持配置拓扑同步间隔
- 支持禁用/启用拓扑同步
- 易于添加新的路由算法

## 测试建议

### 基础测试

```bash
# 1. 编译所有程序
make build

# 2. 启动两个节点
bin/spf_route -node nodeA &
bin/spf_route -node nodeB &

# 3. 添加链路
bin/control -server localhost:5001 -cmd addlink \
  -neighbor nodeB -neighbor-addr localhost:5002

# 4. 运行业务应用示例
bin/simple_sender
```

### 多跳转发测试

```bash
# 启动 A -> B -> C 三节点链式拓扑
# 测试从 A 到 C 的多跳转发
```

### 动态拓扑测试

```bash
# 测试节点动态加入和退出
# 观察路由表的自动更新
```

## 总结

本项目**完整实现**了课题要求的所有功能：

✅ **路由层**：收集节点信息、SPF 算法、路由表维护  
✅ **数据传输层**：便捷的 SDK 接口、屏蔽网络复杂性  
✅ **最短路径传输**：数据自动沿最优路径到达目的节点  

项目代码结构清晰、文档完善、易于使用和扩展，完全符合分布式数据传输框架的设计目标。
