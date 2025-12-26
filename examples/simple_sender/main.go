package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"spfnet/spfnet"
)

func main() {
	// 创建应用节点实例
	c, err := spfnet.NewNode(spfnet.Config{
		NodeID:   "myApp",
		NodeIP:   "127.0.0.1",
		GRPCPort: 5003,
		SerfPort: 7003,
		JoinAddr: "127.0.0.1:7001", // 加入已有集群
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// 启动客户端
	if err := c.Start(); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}
	defer c.Stop()

	log.Println("Client started, waiting for topology sync...")
	time.Sleep(2 * time.Second)

	// 添加链路（可选，如果需要直接连接到某个节点）
	if err := c.AddLink("nodeA", "127.0.0.1:5001", 0); err != nil {
		log.Printf("Warning: Failed to add link: %v", err)
	}

	// 发送数据到目标节点
	log.Println("Sending packet to nodeA...")
	if err := c.Send("nodeA", []byte("Hello from business app!")); err != nil {
		log.Printf("Failed to send packet: %v", err)
	}

	// 持续发送数据
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			// 每 5 秒发送一次数据
			log.Println("Sending periodic packet...")
			if err := c.Send("nodeA", []byte("Periodic message")); err != nil {
				log.Printf("Failed to send packet: %v", err)
			}

		case <-sigCh:
			log.Println("Shutting down...")
			return
		}
	}
}
