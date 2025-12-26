package route

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	pb "spfnet/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RouteNode 代表整个路由节点应用
type RouteNode struct {
	config         *RuntimeConfig
	node           *Node
	routeManager   *RouteManager
	forwardManager *ForwardManager
	topologySync   *TopologySync
	grpcServer     *grpc.Server
}

// NewRouteNode 创建一个新的 RouteNode 实例
func NewRouteNode(config *RuntimeConfig) *RouteNode {
	return &RouteNode{
		config: config,
	}
}

// Init 初始化应用组件
func (n *RouteNode) Init() error {
	// 1. 设置日志输出
	if err := n.config.SetupLogger(); err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}

	log.Printf("=== Node Initialization ===")
	log.Printf("Node: %s | IP: %s | gRPC: %d | Serf: %d\n",
		n.config.NodeID, n.config.NodeIP, n.config.GRPCPort, n.config.SerfPort)

	// 2. 创建核心组件
	n.node = NewNode(n.config.NodeID, n.config.NodeIP, n.config.GRPCPort)
	topology := n.node.GetTopology()

	n.routeManager = NewRouteManager(n.config.NodeID, topology)
	n.forwardManager = NewForwardManager(n.config.NodeID, topology, n.routeManager)
	n.topologySync = NewTopologySync(n.node, topology)

	// 3. 应用拓扑配置
	if n.config.AppConfig.Topology.SyncInterval > 0 {
		syncInterval := time.Duration(n.config.AppConfig.Topology.SyncInterval) * time.Second
		n.topologySync.SetSyncInterval(syncInterval)
		log.Printf("[%s] Set topology sync interval to %v", n.config.NodeID, syncInterval)
	}

	// 4. 设置拓扑变化回调
	n.topologySync.SetTopologyChangeCallback(func() {
		log.Printf("\n[%s] ⚡ Topology Changed!", n.config.NodeID)
		log.Println(topology.String())

		// 重新计算路由
		if err := n.routeManager.RecomputeRoutes(); err != nil {
			log.Printf("Failed to recompute routes: %v", err)
		}
	})

	return nil
}

// Start 启动应用
func (n *RouteNode) Start() error {
	// 1. 启动 Serf（加入集群）
	bindAddr := fmt.Sprintf("%s:%d", n.config.NodeIP, n.config.SerfPort)
	var joinAddrs []string
	if n.config.JoinAddr != "" {
		joinAddrs = []string{n.config.JoinAddr}
		log.Printf("[%s] Joining cluster at %s", n.config.NodeID, n.config.JoinAddr)
	}

	if err := n.node.Start(bindAddr, joinAddrs); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	// 2. 加载边配置
	topology := n.node.GetTopology()
	if len(n.config.Edges) > 0 {
		log.Printf("[%s] Loading %d edges from config...", n.config.NodeID, len(n.config.Edges))
		for _, edge := range n.config.Edges {
			topology.UpdateLink(edge.From, edge.To, float64(edge.Cost))
			log.Printf("[%s] Configured edge: %s-%s cost=%d", n.config.NodeID, edge.From, edge.To, edge.Cost)
		}
	}

	// 3. 启动拓扑同步
	n.topologySync.Start()

	// 4. 启动 gRPC 服务器
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", n.config.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	n.grpcServer = grpc.NewServer()
	pb.RegisterNodeServiceServer(n.grpcServer, NewNodeServer(n.config.NodeID, n.forwardManager))
	pb.RegisterControlServiceServer(n.grpcServer, NewControlServer(n.config.NodeID, topology, n.forwardManager, n.topologySync))

	go func() {
		log.Printf("[%s] gRPC server started on :%d\n", n.config.NodeID, n.config.GRPCPort)
		if err := n.grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// 5. 打印初始拓扑
	time.Sleep(1 * time.Second)
	log.Printf("\n[%s] Initial Topology:", n.config.NodeID)
	log.Println(topology.String())

	// 6. 定期打印拓扑
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			log.Printf("\n[%s] Current Topology:", n.config.NodeID)
			log.Println(topology.String())
		}
	}()

	return nil
}

// Stop 停止应用
func (n *RouteNode) Stop() {
	log.Printf("\n[%s] Shutting down...", n.config.NodeID)
	if n.grpcServer != nil {
		n.grpcServer.GracefulStop()
	}
	log.Printf("[%s] Shutdown complete", n.config.NodeID)
}

// EnableSync 启用或禁用拓扑同步
func (n *RouteNode) EnableSync(enabled bool) error {
	if n.topologySync == nil {
		return fmt.Errorf("topology sync not initialized")
	}

	n.topologySync.EnableSync(enabled)
	log.Printf("[%s] Topology sync %s", n.config.NodeID, map[bool]string{true: "enabled", false: "disabled"}[enabled])
	return nil
}

// IsSyncEnabled 检查拓扑同步是否启用
func (n *RouteNode) IsSyncEnabled() bool {
	if n.topologySync == nil {
		return false
	}
	return n.topologySync.IsSyncEnabled()
}

// SendPacket 发送数据包到目标节点
func (n *RouteNode) SendPacket(ctx context.Context, destination string, payload []byte) error {
	if n.forwardManager == nil {
		return fmt.Errorf("forward manager not initialized")
	}
	return n.forwardManager.SendPacket(ctx, destination, payload)
}

// AddLink 添加到邻居节点的链路
func (n *RouteNode) AddLink(ctx context.Context, neighborID, neighborAddr string, cost float64, autoProbe bool) error {
	if n.node == nil {
		return fmt.Errorf("node not initialized")
	}

	topology := n.node.GetTopology()

	// 添加邻居节点到拓扑
	neighborNode := &NodeInfo{
		ID:      neighborID,
		RPCAddr: neighborAddr,
		Status:  NodeStatusUnknown,
	}
	topology.AddNode(neighborNode)

	// 确定链路成本
	finalCost := cost
	if autoProbe || finalCost <= 0 {
		// 自动探测链路质量
		probedCost, err := n.probeLinkCost(ctx, neighborAddr)
		if err != nil {
			return fmt.Errorf("failed to probe link: %w", err)
		}
		finalCost = probedCost
		log.Printf("[%s] Auto-probed link cost to %s: %.2f", n.config.NodeID, neighborID, finalCost)
	}

	// 更新拓扑中的链路
	topology.UpdateLink(n.config.NodeID, neighborID, finalCost)

	// 通过 Serf 广播链路更新事件
	if n.topologySync != nil {
		if err := n.topologySync.RegisterLink(n.config.NodeID, neighborID, finalCost); err != nil {
			log.Printf("[%s] Warning: Failed to broadcast link update: %v", n.config.NodeID, err)
		}
	}

	log.Printf("[%s] ✓ Successfully added link to %s with cost %.2f", n.config.NodeID, neighborID, finalCost)
	return nil
}

// probeLinkCost 探测链路成本
func (n *RouteNode) probeLinkCost(ctx context.Context, targetAddr string) (float64, error) {
	conn, err := grpc.Dial(targetAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return 0, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := pb.NewNodeServiceClient(conn)
	resp, err := client.ProbeLinkQuality(ctx, &pb.ProbeRequest{
		Source:    n.config.NodeID,
		Target:    "",
		SelfDebug: true,
	})
	if err != nil {
		return 0, fmt.Errorf("probe request failed: %w", err)
	}

	if !resp.Success {
		return 0, fmt.Errorf("probe failed: %s", resp.Message)
	}

	return resp.Cost, nil
}
