package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	pb "spfnet/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	serverAddr = flag.String("server", "localhost:5001", "Server address (ip:port)")
	command    = flag.String("cmd", "", "Command to execute: addlink, ping, sendpacket")

	// addlink 参数
	neighbor        = flag.String("neighbor", "", "Neighbor node ID")
	neighborAddress = flag.String("neighbor-addr", "", "Neighbor gRPC address (ip:port)")
	cost            = flag.Float64("cost", 0, "Link cost (0 for auto-probe)")
	autoProbe       = flag.Bool("auto-probe", true, "Auto probe link quality")

	// sendpacket 参数
	sourceAddr  = flag.String("source-addr", "", "Source node gRPC address (ip:port)")
	sourceNode  = flag.String("source", "", "Source node ID")
	destNode    = flag.String("dest", "", "Destination node ID")
	payload     = flag.String("payload", "hello", "Packet payload")
	packetID    = flag.String("packet-id", "", "Packet ID (auto-generated if empty)")

	// enablesync 参数
	syncEnabled = flag.Bool("enabled", true, "Enable or disable sync")
)

func main() {
	flag.Parse()

	if *command == "" {
		fmt.Fprintf(os.Stderr, "Error: -cmd is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// 建立 gRPC 连接
	conn, err := grpc.NewClient(*serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	client := pb.NewControlServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch *command {
	case "ping":
		doPing(ctx, client)
	case "addlink":
		doAddLink(ctx, client)
	case "sendpacket":
		doSendPacket(ctx, client)
	case "enablesync":
		doEnableSync(ctx, client)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", *command)
		fmt.Fprintf(os.Stderr, "Available commands: ping, addlink, sendpacket, enablesync\n")
		os.Exit(1)
	}
}

func doPing(ctx context.Context, client pb.ControlServiceClient) {
	fmt.Printf("Sending ping to %s...\n", *serverAddr)

	resp, err := client.Ping(ctx, &pb.PingRequest{Msg: "ping"})
	if err != nil {
		log.Fatalf("Ping failed: %v", err)
	}

	fmt.Printf("✓ %s\n", resp.Msg)
}

func doAddLink(ctx context.Context, client pb.ControlServiceClient) {
	if *neighbor == "" {
		fmt.Fprintf(os.Stderr, "Error: -neighbor is required for addlink command\n")
		os.Exit(1)
	}

	if *neighborAddress == "" {
		fmt.Fprintf(os.Stderr, "Error: -neighbor-addr is required for addlink command\n")
		os.Exit(1)
	}

	fmt.Printf("Adding link to neighbor %s (%s)...\n", *neighbor, *neighborAddress)
	fmt.Printf("  Cost: %.2f, Auto-probe: %v\n", *cost, *autoProbe)

	req := &pb.AddLinkRequest{
		Neighbor:        *neighbor,
		NeighborAddress: *neighborAddress,
		Cost:            *cost,
		AutoProbe:       *autoProbe,
	}

	resp, err := client.AddLink(ctx, req)
	if err != nil {
		log.Fatalf("AddLink failed: %v", err)
	}

	if resp.Success {
		fmt.Printf("✓ %s\n", resp.Message)
		fmt.Printf("  Final cost: %.2f\n", resp.Cost)
	} else {
		fmt.Printf("✗ Failed: %s\n", resp.Message)
		os.Exit(1)
	}
}

func doSendPacket(ctx context.Context, client pb.ControlServiceClient) {
	if *sourceAddr == "" {
		fmt.Fprintf(os.Stderr, "Error: -source-addr is required for sendpacket command\n")
		os.Exit(1)
	}

	if *sourceNode == "" {
		fmt.Fprintf(os.Stderr, "Error: -source is required for sendpacket command\n")
		os.Exit(1)
	}

	if *destNode == "" {
		fmt.Fprintf(os.Stderr, "Error: -dest is required for sendpacket command\n")
		os.Exit(1)
	}

	fmt.Printf("Sending packet from %s to %s...\n", *sourceNode, *destNode)
	fmt.Printf("  Source address: %s\n", *sourceAddr)
	fmt.Printf("  Payload: %s\n", *payload)

	packet := &pb.Packet{
		Source:      *sourceNode,
		Destination: *destNode,
		PacketId:    *packetID,
		Payload:     []byte(*payload),
	}

	req := &pb.SendPacketRequest{
		SourceAddress: *sourceAddr,
		Packet:        packet,
	}

	resp, err := client.SendPacket(ctx, req)
	if err != nil {
		log.Fatalf("SendPacket failed: %v", err)
	}

	if resp.Success {
		fmt.Printf("✓ %s\n", resp.Message)
		fmt.Printf("  Packet ID: %s\n", resp.PacketId)
	} else {
		fmt.Printf("✗ Failed: %s\n", resp.Message)
		os.Exit(1)
	}
}

func doEnableSync(ctx context.Context, client pb.ControlServiceClient) {
	fmt.Printf("Setting sync state to %v on %s...\n", *syncEnabled, *serverAddr)

	req := &pb.EnableSyncRequest{
		Enabled: *syncEnabled,
	}

	resp, err := client.EnableSync(ctx, req)
	if err != nil {
		log.Fatalf("EnableSync failed: %v", err)
	}

	if resp.Success {
		fmt.Printf("✓ %s\n", resp.Message)
		fmt.Printf("  Current state: enabled=%v\n", resp.Enabled)
	} else {
		fmt.Printf("✗ Failed: %s\n", resp.Message)
		os.Exit(1)
	}
}
