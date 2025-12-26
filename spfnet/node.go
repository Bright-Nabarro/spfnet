package spfnet

import (
	"context"
	"fmt"
	"time"

	"spfnet/internal/route"
)

// Node 是对外提供的应用节点
// 封装了复杂的网络和路由逻辑，业务层只需调用简单的 API
type Node struct {
	routeNode *route.RouteNode
}

// Config 应用节点配置
type Config struct {
	// 必填项
	NodeID   string // 节点 ID，如 "nodeA"
	NodeIP   string // 节点 IP，如 "127.0.0.1"
	GRPCPort int    // gRPC 端口，如 5001
	SerfPort int    // Serf 端口，如 7001

	// 可选项
	AppConfigPath string // 应用配置文件路径，默认 "configs/app.toml"
	JoinAddr      string // 加入的集群地址，如 "127.0.0.1:7001"
}

// NewNode 创建一个新的应用节点实例
// 示例：
//
//	node, err := spfnet.NewNode(spfnet.Config{
//	    NodeID:   "nodeA",
//	    NodeIP:   "127.0.0.1",
//	    GRPCPort: 5001,
//	    SerfPort: 7001,
//	})
func NewNode(cfg Config) (*Node, error) {
	// 设置默认值
	if cfg.AppConfigPath == "" {
		cfg.AppConfigPath = "configs/app.toml"
	}

	// 加载运行时配置
	rtConfig, err := route.LoadRuntimeConfig(
		cfg.AppConfigPath,
		"",           // 不使用配置文件方式
		"",           // 不使用配置文件方式
		cfg.NodeID,   // 手动指定 ID
		cfg.NodeIP,   // 手动指定 IP
		cfg.GRPCPort, // 手动指定端口
		cfg.SerfPort, // 手动指定 Serf 端口
		cfg.JoinAddr, // 加入地址
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime config: %w", err)
	}

	// 创建路由节点
	routeNode := route.NewRouteNode(rtConfig)
	if err := routeNode.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize node: %w", err)
	}

	return &Node{
		routeNode: routeNode,
	}, nil
}

// Start 启动应用节点
// 启动后节点会自动：
// - 加入集群
// - 同步拓扑信息
// - 计算最短路径
// - 启动 gRPC 服务
func (n *Node) Start() error {
	return n.routeNode.Start()
}

// Stop 停止应用节点
func (n *Node) Stop() {
	n.routeNode.Stop()
}

// Send 发送数据到目标节点
// 参数：
//   - destination: 目标节点 ID，如 "nodeC"
//   - data: 要发送的数据
//
// 示例：
//
//	err := node.Send("nodeC", []byte("hello world"))
func (n *Node) Send(destination string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return n.routeNode.SendPacket(ctx, destination, data)
}

// SendWithContext 使用自定义 context 发送数据
func (n *Node) SendWithContext(ctx context.Context, destination string, data []byte) error {
	return n.routeNode.SendPacket(ctx, destination, data)
}

// AddLink 添加到邻居节点的链路
// 参数：
//   - neighborID: 邻居节点 ID
//   - neighborAddr: 邻居节点地址，格式 "ip:port"
//   - cost: 链路成本，0 表示自动探测
//
// 示例：
//
//	err := node.AddLink("nodeB", "127.0.0.1:5002", 0)
func (n *Node) AddLink(neighborID, neighborAddr string, cost float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return n.routeNode.AddLink(ctx, neighborID, neighborAddr, cost, true)
}

// EnableSync 启用或禁用拓扑同步
func (n *Node) EnableSync(enabled bool) error {
	return n.routeNode.EnableSync(enabled)
}

// IsSyncEnabled 检查拓扑同步是否启用
func (n *Node) IsSyncEnabled() bool {
	return n.routeNode.IsSyncEnabled()
}
