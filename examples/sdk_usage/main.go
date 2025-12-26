package main

import (
	"log"
	"time"

	"spfnet/spfnet"
)

// 示例 1: 基本使用
func basicUsage() {
	// 创建应用节点实例
	c, err := spfnet.NewNode(spfnet.Config{
		NodeID:   "app1",
		NodeIP:   "127.0.0.1",
		GRPCPort: 5010,
		SerfPort: 7010,
		JoinAddr: "127.0.0.1:7001", // 加入现有集群
	})
	if err != nil {
		log.Fatal(err)
	}

	// 启动客户端
	if err := c.Start(); err != nil {
		log.Fatal(err)
	}
	defer c.Stop()

	// 等待拓扑同步
	time.Sleep(2 * time.Second)

	// 发送数据
	if err := c.Send("nodeA", []byte("Hello from business app")); err != nil {
		log.Printf("Send failed: %v", err)
	}

	log.Println("Message sent successfully!")
}

// 示例 2: 添加链路后发送数据
func addLinkAndSend() {
	c, err := spfnet.NewNode(spfnet.Config{
		NodeID:   "app2",
		NodeIP:   "127.0.0.1",
		GRPCPort: 5011,
		SerfPort: 7011,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := c.Start(); err != nil {
		log.Fatal(err)
	}
	defer c.Stop()

	// 添加到邻居的链路
	if err := c.AddLink("nodeA", "127.0.0.1:5001", 0); err != nil {
		log.Printf("AddLink failed: %v", err)
		return
	}

	time.Sleep(1 * time.Second)

	// 发送数据
	if err := c.Send("nodeA", []byte("Direct message")); err != nil {
		log.Printf("Send failed: %v", err)
		return
	}

	log.Println("Direct message sent!")
}

// 示例 3: 向多个节点发送数据
func multipleNodes() {
	c, err := spfnet.NewNode(spfnet.Config{
		NodeID:   "app3",
		NodeIP:   "127.0.0.1",
		GRPCPort: 5012,
		SerfPort: 7012,
		JoinAddr: "127.0.0.1:7001",
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := c.Start(); err != nil {
		log.Fatal(err)
	}
	defer c.Stop()

	time.Sleep(2 * time.Second)

	// 向多个目标发送数据
	targets := []string{"nodeA", "nodeB", "nodeC"}
	for _, target := range targets {
		if err := c.Send(target, []byte("Broadcast message")); err != nil {
			log.Printf("Failed to send to %s: %v", target, err)
		} else {
			log.Printf("Sent to %s successfully", target)
		}
	}
}

func main() {
	log.Println("=== SPFNet SDK Usage Examples ===")
	log.Println("\nRunning Example 1: Basic Usage")
	basicUsage()

	// 如果需要运行其他示例，取消注释：
	// log.Println("\nRunning Example 2: Add Link and Send")
	// addLinkAndSend()

	// log.Println("\nRunning Example 3: Multiple Nodes")
	// multipleNodes()
}
