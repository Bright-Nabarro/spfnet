package route

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "spfnet/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NodeServer 实现 gRPC 服务
type NodeServer struct {
	pb.UnimplementedNodeServiceServer
	NodeID         string
	ForwardManager *ForwardManager
}

// ControlServer 实现控制管理服务
type ControlServer struct {
	pb.UnimplementedControlServiceServer
	NodeID         string
	Topology       *Topology
	ForwardManager *ForwardManager
	TopologySync   *TopologySync
}

func NewNodeServer(nodeID string, forwardManager *ForwardManager) *NodeServer {
	return &NodeServer{
		NodeID:         nodeID,
		ForwardManager: forwardManager,
	}
}

func NewControlServer(nodeID string, topology *Topology, forwardManager *ForwardManager, topologySync *TopologySync) *ControlServer {
	return &ControlServer{
		NodeID:         nodeID,
		Topology:       topology,
		ForwardManager: forwardManager,
		TopologySync:   topologySync,
	}
}

// ============ NodeServer 方法 ============

func (s *NodeServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Msg: fmt.Sprintf("pong from %s", s.NodeID)}, nil
}

func (s *NodeServer) ForwardPacket(ctx context.Context, packet *pb.Packet) (*pb.ForwardResponse, error) {
	return s.ForwardManager.HandleIncomingPacket(ctx, packet)
}

func (s *NodeServer) ProbeLinkQuality(ctx context.Context, req *pb.ProbeRequest) (*pb.ProbeResponse, error) {
	if req.SelfDebug {
		// 调试模式：随机生成合理范围的值
		// RTT: 1-50ms, Cost: 1-20
		rttMs := int64(1 + rand.Intn(50))
		cost := 1.0 + rand.Float64()*19.0 // 1.0 ~ 20.0

		return &pb.ProbeResponse{
			Success: true,
			RttMs:   rttMs,
			Cost:    cost,
		}, nil
	}

	// 生产模式：实际测量 RTT
	panic("TODO: implement real link quality probe with actual RTT measurement")
}

// ============ ControlServer 方法 ============

func (s *ControlServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Msg: fmt.Sprintf("pong from %s (control)", s.NodeID)}, nil
}

func (s *ControlServer) SendPacket(ctx context.Context, req *pb.SendPacketRequest) (*pb.SendPacketResponse, error) {
	log.Printf("[%s] Received SendPacket request: source_address=%s, packet=%+v",
		s.NodeID, req.SourceAddress, req.Packet)

	// 验证参数
	if req.SourceAddress == "" {
		return &pb.SendPacketResponse{
			Success: false,
			Message: "source address cannot be empty",
		}, nil
	}

	if req.Packet == nil {
		return &pb.SendPacketResponse{
			Success: false,
			Message: "packet cannot be nil",
		}, nil
	}

	// 生成 packet ID（如果没有提供）
	packetID := req.Packet.PacketId
	if packetID == "" {
		packetID = fmt.Sprintf("pkt-%d-%s", time.Now().UnixNano(), s.NodeID)
		req.Packet.PacketId = packetID
	}

	// 建立到源节点的 gRPC 连接
	conn, err := grpc.Dial(req.SourceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.SendPacketResponse{
			Success:  false,
			Message:  fmt.Sprintf("failed to connect to source node: %v", err),
			PacketId: packetID,
		}, nil
	}
	defer conn.Close()

	client := pb.NewNodeServiceClient(conn)

	// 发送数据包
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.ForwardPacket(sendCtx, req.Packet)
	if err != nil {
		return &pb.SendPacketResponse{
			Success:  false,
			Message:  fmt.Sprintf("failed to send packet: %v", err),
			PacketId: packetID,
		}, nil
	}

	if !resp.Success {
		return &pb.SendPacketResponse{
			Success:  false,
			Message:  fmt.Sprintf("packet forwarding failed: %s", resp.Message),
			PacketId: packetID,
		}, nil
	}

	log.Printf("[%s] ✓ Successfully sent packet %s from %s to %s",
		s.NodeID, packetID, req.Packet.Source, req.Packet.Destination)

	return &pb.SendPacketResponse{
		Success:  true,
		Message:  fmt.Sprintf("packet sent successfully: %s", resp.Message),
		PacketId: packetID,
	}, nil
}

func (s *ControlServer) AddLink(ctx context.Context, req *pb.AddLinkRequest) (*pb.AddLinkResponse, error) {
	log.Printf("[%s] Received AddLink request: neighbor=%s, address=%s, cost=%.2f, auto_probe=%v",
		s.NodeID, req.Neighbor, req.NeighborAddress, req.Cost, req.AutoProbe)

	// 验证参数
	if req.Neighbor == "" {
		return &pb.AddLinkResponse{
			Success: false,
			Message: "neighbor ID cannot be empty",
		}, nil
	}

	if req.NeighborAddress == "" {
		return &pb.AddLinkResponse{
			Success: false,
			Message: "neighbor address cannot be empty",
		}, nil
	}

	// 添加邻居节点到拓扑
	neighborNode := &NodeInfo{
		ID:      req.Neighbor,
		RPCAddr: req.NeighborAddress,
		Status:  NodeStatusUnknown,
	}
	s.Topology.AddNode(neighborNode)

	// 确定链路成本
	finalCost := req.Cost
	if req.AutoProbe || finalCost <= 0 {
		// 自动探测链路质量
		probedCost, err := s.probeLinkCost(ctx, req.NeighborAddress)
		if err != nil {
			log.Printf("[%s] Failed to probe link to %s: %v", s.NodeID, req.Neighbor, err)
			return &pb.AddLinkResponse{
				Success: false,
				Message: fmt.Sprintf("failed to probe link: %v", err),
			}, nil
		}
		finalCost = probedCost
		log.Printf("[%s] Auto-probed link cost to %s: %.2f", s.NodeID, req.Neighbor, finalCost)
	}

	// 更新拓扑中的链路
	s.Topology.UpdateLink(s.NodeID, req.Neighbor, finalCost)

	// 通过 Serf 广播链路更新事件，让集群中的其他节点知道
	if s.TopologySync != nil {
		if err := s.TopologySync.RegisterLink(s.NodeID, req.Neighbor, finalCost); err != nil {
			log.Printf("[%s] Warning: Failed to broadcast link update: %v", s.NodeID, err)
			// 不返回错误，因为本地拓扑已经更新成功
		}
	}

	log.Printf("[%s] ✓ Successfully added link to %s with cost %.2f", s.NodeID, req.Neighbor, finalCost)

	return &pb.AddLinkResponse{
		Success: true,
		Message: fmt.Sprintf("link added: %s -> %s (cost: %.2f)", s.NodeID, req.Neighbor, finalCost),
		Cost:    finalCost,
	}, nil
}

func (s *ControlServer) EnableSync(ctx context.Context, req *pb.EnableSyncRequest) (*pb.EnableSyncResponse, error) {
	log.Printf("[%s] Received EnableSync request: enabled=%v", s.NodeID, req.Enabled)

	if s.TopologySync == nil {
		return &pb.EnableSyncResponse{
			Success: false,
			Message: "topology sync is not initialized",
			Enabled: false,
		}, nil
	}

	// 设置同步状态
	s.TopologySync.EnableSync(req.Enabled)

	// 获取当前状态
	currentState := s.TopologySync.IsSyncEnabled()

	message := "topology sync disabled"
	if currentState {
		message = "topology sync enabled"
	}

	log.Printf("[%s] ✓ Topology sync state changed: enabled=%v", s.NodeID, currentState)

	return &pb.EnableSyncResponse{
		Success: true,
		Message: message,
		Enabled: currentState,
	}, nil
}

// probeLinkCost 探测链路成本
func (s *ControlServer) probeLinkCost(ctx context.Context, address string) (float64, error) {
	// 建立 gRPC 连接
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return 0, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := pb.NewNodeServiceClient(conn)

	// 发送探测请求
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.ProbeLinkQuality(probeCtx, &pb.ProbeRequest{
		Source:    s.NodeID,
		Target:    "",
		SelfDebug: true, // 使用调试模式
	})

	if err != nil {
		return 0, fmt.Errorf("probe failed: %w", err)
	}

	if !resp.Success {
		return 0, fmt.Errorf("probe unsuccessful: %s", resp.Message)
	}

	return resp.Cost, nil
}
