package route

import (
	"context"
	"fmt"
	"log"
	"sync"

	pb "spfnet/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RouteManager 路由管理器
type RouteManager struct {
	nodeID     string
	topology   *Topology
	routeTable *RouteTable
	spfCalc    *SPFCalculator
	mtx        sync.RWMutex
}

// NewRouteManager 创建路由管理器
func NewRouteManager(nodeID string, topology *Topology) *RouteManager {
	return &RouteManager{
		nodeID:     nodeID,
		topology:   topology,
		routeTable: NewRouteTable(nodeID),
		spfCalc:    NewSPFCalculator(),
	}
}

// RecomputeRoutes 重新计算路由表
func (rm *RouteManager) RecomputeRoutes() error {
	rm.mtx.Lock()
	defer rm.mtx.Unlock()

	log.Printf("[%s] Recomputing routes...", rm.nodeID)

	// 使用 SPF 算法计算路由
	newRouteTable, err := rm.spfCalc.ComputeRoutes(rm.nodeID, rm.topology)
	if err != nil {
		return fmt.Errorf("failed to compute routes: %w", err)
	}

	// 更新路由表
	rm.routeTable = newRouteTable

	log.Printf("[%s] Route table updated:\n%s", rm.nodeID, rm.routeTable.String())
	return nil
}

// GetRoute 获取到指定目的地的路由
func (rm *RouteManager) GetRoute(destination string) (*Route, error) {
	rm.mtx.RLock()
	defer rm.mtx.RUnlock()

	return rm.routeTable.GetRoute(destination)
}

// GetRouteTable 获取当前路由表
func (rm *RouteManager) GetRouteTable() *RouteTable {
	rm.mtx.RLock()
	defer rm.mtx.RUnlock()

	return rm.routeTable
}

// GetNextHop 获取到目的地的下一跳
func (rm *RouteManager) GetNextHop(destination string) (string, error) {
	rm.mtx.RLock()
	defer rm.mtx.RUnlock()

	return rm.routeTable.GetNextHop(destination)
}

// GetGRPCClient 获取到指定节点的 gRPC 客户端连接
func (rm *RouteManager) GetGRPCClient(nodeInfo *NodeInfo) (pb.NodeServiceClient, *grpc.ClientConn, error) {
	addr := fmt.Sprintf("%s:%d", nodeInfo.IP, nodeInfo.Port)
	
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	client := pb.NewNodeServiceClient(conn)
	return client, conn, nil
}

// SendPacket 发送数据包（根据路由表转发到下一跳）
func (rm *RouteManager) SendPacket(ctx context.Context, packet *pb.Packet) error {
	// 查询路由
	route, err := rm.GetRoute(packet.Destination)
	if err != nil {
		return fmt.Errorf("no route to %s: %w", packet.Destination, err)
	}

	// 更新下一跳
	packet.NextHop = route.NextHop

	// 获取下一跳节点信息
	nextHopNode := rm.topology.GetNode(route.NextHop)
	if nextHopNode == nil {
		return fmt.Errorf("next hop node %s not found", route.NextHop)
	}

	// 连接到下一跳节点
	addr := fmt.Sprintf("%s:%d", nextHopNode.IP, nextHopNode.Port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	client := pb.NewNodeServiceClient(conn)

	// 转发数据包
	resp, err := client.ForwardPacket(ctx, packet)
	if err != nil {
		return fmt.Errorf("failed to forward packet: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("forward failed: %s", resp.Message)
	}

	log.Printf("[%s] Packet %s forwarded to %s", rm.nodeID, packet.PacketId, route.NextHop)
	return nil
}

// Close 关闭路由管理器
func (rm *RouteManager) Close() error {
	log.Printf("[%s] Route manager closed", rm.nodeID)
	return nil
}
