package route

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// LogConfig 日志配置
type LogConfig struct {
	Output string `toml:"output"`  // "file" 或 "stdout"
	LogDir string `toml:"log_dir"` // 日志目录
	Level  string `toml:"level"`   // 日志级别
}

// TopologyConfig 拓扑配置
type TopologyConfig struct {
	SyncInterval int `toml:"sync_interval"` // 全量拓扑同步间隔（秒）
}

// AppConfig 应用通用配置
type AppConfig struct {
	Log      LogConfig      `toml:"log"`
	Topology TopologyConfig `toml:"topology"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	ID       string `toml:"id"`
	IP       string `toml:"ip"`
	GRPCPort int    `toml:"grpc_port"`
	SerfPort int    `toml:"serf_port"`
	Join     string `toml:"join"`
}

// EdgeConfig 边配置
type EdgeConfig struct {
	From string `toml:"from"`
	To   string `toml:"to"`
	Cost int    `toml:"cost"`
}

// Config 配置文件结构
type Config struct {
	Nodes []NodeConfig `toml:"nodes"`
	Edges []EdgeConfig `toml:"edges"`
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// GetNodeConfig 根据节点ID获取配置
func (c *Config) GetNodeConfig(nodeID string) (*NodeConfig, error) {
	for _, node := range c.Nodes {
		if node.ID == nodeID {
			return &node, nil
		}
	}
	return nil, fmt.Errorf("node %s not found in config", nodeID)
}

// GetNodeEdges 获取与指定节点相关的所有边（无向图，检查from和to）
func (c *Config) GetNodeEdges(nodeID string) []EdgeConfig {
	var edges []EdgeConfig
	for _, edge := range c.Edges {
		if edge.From == nodeID || edge.To == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

// LoadAppConfig 加载应用通用配置
func LoadAppConfig(filename string) (*AppConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read app config file: %w", err)
	}

	var config AppConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse app config file: %w", err)
	}

	// 设置默认值
	if config.Log.Output == "" {
		config.Log.Output = "file"
	}
	if config.Log.LogDir == "" {
		config.Log.LogDir = "log"
	}
	if config.Topology.SyncInterval == 0 {
		config.Topology.SyncInterval = 30
	}

	return &config, nil
}

// RuntimeConfig 运行时配置（合并了配置文件和命令行参数）
type RuntimeConfig struct {
	NodeID      string
	NodeIP      string
	GRPCPort    int
	SerfPort    int
	JoinAddr    string
	Edges       []EdgeConfig
	AppConfig   *AppConfig
	logFile     *os.File // 用于延迟关闭日志文件
}

// LoadRuntimeConfig 加载运行时配置
// 如果指定了 nodeName，则从配置文件加载；否则使用命令行参数
func LoadRuntimeConfig(
	appConfigPath string,
	topologyConfigPath string,
	nodeName string,
	nodeID string,
	nodeIP string,
	nodePort int,
	serfPort int,
	join string,
) (*RuntimeConfig, error) {
	// 加载应用配置
	appCfg, err := LoadAppConfig(appConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load app config: %v, using defaults\n", err)
		// 使用默认配置
		appCfg = &AppConfig{
			Log: LogConfig{
				Output: "file",
				LogDir: "log",
			},
			Topology: TopologyConfig{
				SyncInterval: 30,
			},
		}
	}

	rc := &RuntimeConfig{
		AppConfig: appCfg,
	}

	// 判断使用哪种配置方式：主要看是否指定了 -node 参数
	if nodeName != "" {
		// 从配置文件读取
		fmt.Printf("Loading config from %s for node %s...\n", topologyConfigPath, nodeName)
		cfg, err := LoadConfig(topologyConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}

		nodeConfig, err := cfg.GetNodeConfig(nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to get node config: %w", err)
		}

		rc.NodeID = nodeConfig.ID
		rc.NodeIP = nodeConfig.IP
		rc.GRPCPort = nodeConfig.GRPCPort
		rc.SerfPort = nodeConfig.SerfPort
		rc.JoinAddr = nodeConfig.Join

		// 加载边配置
		rc.Edges = cfg.GetNodeEdges(rc.NodeID)

		fmt.Printf("✓ Loaded config: ID=%s, IP=%s, gRPC=%d, Serf=%d, Join=%s\n",
			rc.NodeID, rc.NodeIP, rc.GRPCPort, rc.SerfPort, rc.JoinAddr)
	} else {
		// 使用命令行参数
		rc.NodeID = nodeID
		if rc.NodeID == "" {
			rc.NodeID = fmt.Sprintf("node-%d", nodePort)
		}
		rc.NodeIP = nodeIP
		rc.GRPCPort = nodePort
		rc.SerfPort = serfPort
		rc.JoinAddr = join
	}

	return rc, nil
}

// SetupLogger 根据配置设置日志输出
func (rc *RuntimeConfig) SetupLogger() error {
	if rc.AppConfig.Log.Output == "stdout" {
		// 输出到标准输出
		fmt.Printf("[%s] Logging to stdout\n", rc.NodeID)
		return nil
	}

	// 输出到文件
	logDir := rc.AppConfig.Log.LogDir
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.OpenFile(
		fmt.Sprintf("%s/%s.log", logDir, rc.NodeID),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	rc.logFile = logFile
	fmt.Printf("[%s] Logging to %s/%s.log\n", rc.NodeID, logDir, rc.NodeID)
	return nil
}

// Close 关闭日志文件（如果有）
func (rc *RuntimeConfig) Close() error {
	if rc.logFile != nil {
		return rc.logFile.Close()
	}
	return nil
}
