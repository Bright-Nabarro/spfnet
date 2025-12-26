package route

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "spfnet/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ForwardManager 数据包转发管理器
type ForwardManager struct {
	nodeID       string
	topology     *Topology
	routeManager *RouteManager

	// gRPC 连接池
	connPool map[string]*grpc.ClientConn
	poolMtx  sync.RWMutex

	// 统计信息
	stats ForwardStats
}

// ForwardStats 转发统计
type ForwardStats struct {
	mtx              sync.RWMutex
	PacketsSent      int64
	PacketsReceived  int64
	PacketsForwarded int64
	PacketsDelivered int64
	PacketsDropped   int64
}

// NewForwardManager 创建转发管理器
func NewForwardManager(nodeID string, topology *Topology, routeManager *RouteManager) *ForwardManager {
	return &ForwardManager{
		nodeID:       nodeID,
		topology:     topology,
		routeManager: routeManager,
		connPool:     make(map[string]*grpc.ClientConn),
	}
}

// SendPacket 发送数据包到目的节点
func (fm *ForwardManager) SendPacket(ctx context.Context, destination string, payload []byte) error {
	// 创建数据包
	packet := &pb.Packet{
		Source:       fm.nodeID,
		Destination:  destination,
		PacketId:     fmt.Sprintf("pkt-%s-%d", fm.nodeID, time.Now().UnixNano()),
		Payload:      payload,
		VisitedNodes: []string{fm.nodeID},
	}

	log.Printf("[%s] Sending packet %s to %s (payload: %s)",
		fm.nodeID, packet.PacketId, destination, string(payload))

	// 更新统计
	fm.stats.mtx.Lock()
	fm.stats.PacketsSent++
	fm.stats.mtx.Unlock()

	// 转发数据包
	return fm.forwardPacket(ctx, packet)
}

// forwardPacket 转发数据包到下一跳
func (fm *ForwardManager) forwardPacket(ctx context.Context, packet *pb.Packet) error {
	// 查询路由
	route, err := fm.routeManager.GetRoute(packet.Destination)
	if err != nil {
		log.Printf("[%s] ✗ No route to %s: %v", fm.nodeID, packet.Destination, err)
		fm.stats.mtx.Lock()
		fm.stats.PacketsDropped++
		fm.stats.mtx.Unlock()
		return fmt.Errorf("no route to %s: %w", packet.Destination, err)
	}

	// 设置下一跳
	packet.NextHop = route.NextHop

	// 获取下一跳节点信息
	nextHopNode := fm.topology.GetNode(route.NextHop)
	if nextHopNode == nil {
		log.Printf("[%s] ✗ Next hop node %s not found", fm.nodeID, route.NextHop)
		fm.stats.mtx.Lock()
		fm.stats.PacketsDropped++
		fm.stats.mtx.Unlock()
		return fmt.Errorf("next hop node %s not found", route.NextHop)
	}

	// 获取 gRPC 客户端
	client, err := fm.getClient(nextHopNode)
	if err != nil {
		log.Printf("[%s] ✗ Failed to get client for %s: %v", fm.nodeID, route.NextHop, err)
		fm.stats.mtx.Lock()
		fm.stats.PacketsDropped++
		fm.stats.mtx.Unlock()
		return err
	}

	// 转发数据包
	resp, err := client.ForwardPacket(ctx, packet)
	if err != nil {
		log.Printf("[%s] ✗ Failed to forward packet %s: %v", fm.nodeID, packet.PacketId, err)
		fm.stats.mtx.Lock()
		fm.stats.PacketsDropped++
		fm.stats.mtx.Unlock()
		return fmt.Errorf("failed to forward packet: %w", err)
	}

	if !resp.Success {
		log.Printf("[%s] ✗ Forward failed: %s", fm.nodeID, resp.Message)
		fm.stats.mtx.Lock()
		fm.stats.PacketsDropped++
		fm.stats.mtx.Unlock()
		return fmt.Errorf("forward failed: %s", resp.Message)
	}

	log.Printf("[%s] ✓ Packet %s forwarded to %s (path: %v)",
		fm.nodeID, packet.PacketId, route.NextHop, packet.VisitedNodes)

	fm.stats.mtx.Lock()
	fm.stats.PacketsForwarded++
	fm.stats.mtx.Unlock()

	return nil
}

// HandleIncomingPacket 处理接收到的数据包
func (fm *ForwardManager) HandleIncomingPacket(ctx context.Context, packet *pb.Packet) (*pb.ForwardResponse, error) {
	log.Printf("[%s] Received packet %s from %s to %s",
		fm.nodeID, packet.PacketId, packet.Source, packet.Destination)

	// 更新统计
	fm.stats.mtx.Lock()
	fm.stats.PacketsReceived++
	fm.stats.mtx.Unlock()

	// 记录经过的节点
	packet.VisitedNodes = append(packet.VisitedNodes, fm.nodeID)

	// 如果是目的地，则接收
	if packet.Destination == fm.nodeID {
		log.Printf("[%s] ✓ Packet %s delivered! Path: %v",
			fm.nodeID, packet.PacketId, packet.VisitedNodes)
		log.Printf("[%s] Payload: %s", fm.nodeID, string(packet.Payload))

		fm.stats.mtx.Lock()
		fm.stats.PacketsDelivered++
		fm.stats.mtx.Unlock()

		return &pb.ForwardResponse{
			Success: true,
			Message: "Packet delivered",
		}, nil
	}

	// 否则继续转发
	err := fm.forwardPacket(ctx, packet)
	if err != nil {
		return &pb.ForwardResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ForwardResponse{
		Success: true,
		Message: "Packet forwarded",
	}, nil
}

// getClient 获取或创建到指定节点的 gRPC 客户端
func (fm *ForwardManager) getClient(nodeInfo *NodeInfo) (pb.NodeServiceClient, error) {
	fm.poolMtx.RLock()
	conn, exists := fm.connPool[nodeInfo.ID]
	fm.poolMtx.RUnlock()

	if exists && conn.GetState().String() == "READY" {
		return pb.NewNodeServiceClient(conn), nil
	}

	// 创建新连接
	fm.poolMtx.Lock()
	defer fm.poolMtx.Unlock()

	// 双重检查
	if conn, exists := fm.connPool[nodeInfo.ID]; exists {
		if conn.GetState().String() == "READY" {
			return pb.NewNodeServiceClient(conn), nil
		}
		conn.Close()
	}

	// 使用 RPCAddr 字段，如果为空则尝试组合 IP:Port
	addr := nodeInfo.RPCAddr
	if addr == "" {
		addr = fmt.Sprintf("%s:%d", nodeInfo.IP, nodeInfo.Port)
	}
	
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	fm.connPool[nodeInfo.ID] = conn
	return pb.NewNodeServiceClient(conn), nil
}

// GetStats 获取转发统计
func (fm *ForwardManager) GetStats() ForwardStats {
	fm.stats.mtx.RLock()
	defer fm.stats.mtx.RUnlock()

	return ForwardStats{
		PacketsSent:      fm.stats.PacketsSent,
		PacketsReceived:  fm.stats.PacketsReceived,
		PacketsForwarded: fm.stats.PacketsForwarded,
		PacketsDelivered: fm.stats.PacketsDelivered,
		PacketsDropped:   fm.stats.PacketsDropped,
	}
}

// PrintStats 打印统计信息
func (fm *ForwardManager) PrintStats() {
	stats := fm.GetStats()
	fmt.Printf("\n=== Forward Statistics [%s] ===\n", fm.nodeID)
	fmt.Printf("Packets Sent:      %d\n", stats.PacketsSent)
	fmt.Printf("Packets Received:  %d\n", stats.PacketsReceived)
	fmt.Printf("Packets Forwarded: %d\n", stats.PacketsForwarded)
	fmt.Printf("Packets Delivered: %d\n", stats.PacketsDelivered)
	fmt.Printf("Packets Dropped:   %d\n", stats.PacketsDropped)
	fmt.Printf("================================\n")
}

// Close 关闭转发管理器
func (fm *ForwardManager) Close() error {
	fm.poolMtx.Lock()
	defer fm.poolMtx.Unlock()

	for nodeID, conn := range fm.connPool {
		if err := conn.Close(); err != nil {
			log.Printf("Failed to close connection to %s: %v", nodeID, err)
		}
	}

	fm.connPool = make(map[string]*grpc.ClientConn)
	log.Printf("[%s] Forward manager closed", fm.nodeID)
	return nil
}
