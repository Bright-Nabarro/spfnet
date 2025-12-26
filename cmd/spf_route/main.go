package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"spfnet/internal/route"
)

var ( // 通用配置
	appConfig = flag.String("app-config", "configs/app.toml", "Application config file path")
	// 配置文件方式
	configFile = flag.String("config", "configs/topology.toml", "Config file path")
	nodeName   = flag.String("node", "", "Node name in config file (e.g., nodeA)")

	// 手动指定方式
	nodeID   = flag.String("id", "", "Node ID (default: node-<port>)")
	nodeIP   = flag.String("ip", "127.0.0.1", "Node IP")
	nodePort = flag.Int("port", 5001, "gRPC port")
	serfPort = flag.Int("serf-port", 7001, "Serf port")
	join     = flag.String("join", "", "Address of node to join (e.g., 127.0.0.1:7001)")
)

func main() {
	flag.Parse()

	// 加载运行时配置
	rtConfig, err := route.LoadRuntimeConfig(
		*appConfig,
		*configFile,
		*nodeName,
		*nodeID,
		*nodeIP,
		*nodePort,
		*serfPort,
		*join,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load runtime config: %v\n", err)
		os.Exit(1)
	}
	defer rtConfig.Close()

	// 创建并启动应用
	node := route.NewRouteNode(rtConfig)
	if err := node.Init(); err != nil {
		log.Fatalf("Failed to initialize node: %v", err)
	}

	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	node.Stop()
}
