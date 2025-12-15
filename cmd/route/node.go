package route

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/serf/serf"
)

const (
	EventLinkUpdate = "link-update"
)

type NodeStatus int

const (
	NodeStatusUnknown NodeStatus = iota
	NodeStatusAlive
	NodeStatusSuspect
	NodeStatusFailed
	NodeStatusLeft
)

type Node struct {
	NodeInfo
	// serf 集群
	serf    *serf.Serf
	eventCh chan serf.Event

	topology *Topology

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// 默认无向图
type Topology struct {
	mtx   sync.RWMutex
	nodes map[string]*NodeInfo
	// 边集合 from -> to -> cost
	edges map[string]map[string]float64
}

type NodeInfo struct {
	ID     string
	IP     string
	Port   int
	Status NodeStatus
}

// 链路事件更新
type LinkUpdateEvent struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	Cost float64 `json:"cost"` // 只在 add/update 时有意义
	//Sequence int64   `json:"seq"`
	Op string `json:"op"` // "add", "update", "remove"
}

func NewNode(name, ip string, port int) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	return &Node{
		NodeInfo: NodeInfo{
			ID:   name,
			IP:   ip,
			Port: port,
		},
		eventCh: make(chan serf.Event, 256),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func NewTopology() *Topology {
	return &Topology{
		nodes: make(map[string]*NodeInfo),
		edges: make(map[string]map[string]float64),
	}
}

// 启动节点
func (n *Node) Start(bindAddr string, joinAddrs []string) error {
	// 配置serf
	config := serf.DefaultConfig()
	config.NodeName = n.ID
	config.MemberlistConfig.BindAddr = n.IP
	config.EventCh = n.eventCh
	// 设置节点标签（metadata)
	config.Tags = map[string]string{
		"node_id": n.ID,
		"ip":      n.IP,
		"port":    fmt.Sprintf("%d", n.Port),
		"role":    "spf-node",
	}

	// 创建实例
	s, err := serf.Create(config)
	if err != nil {
		return fmt.Errorf("failed to create serf %w", err)
	}
	n.serf = s

	// 将自己放入拓扑
	n.topology.AddNode(&n.NodeInfo)

	return nil
}

// Topology func

func (t *Topology) AddNode(node *NodeInfo) {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	t.nodes[node.ID] = node
	if t.edges[node.ID] == nil {
		t.edges[node.ID] = make(map[string]float64)
	}
}

func (t *Topology) RemoveNode(nodeName string) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	// 删除节点表中的对应节点
	delete(t.nodes, nodeName)
	// 删除所有指向此节点的边 (无向图)
	neighbors, ok := t.edges[nodeName]
	if ok {
		for neighbor := range neighbors {
			delete(t.edges[neighbor], nodeName)
		}
	}
	// 删除此节点所有的指向的边
	delete(t.edges, nodeName)
}

func (t *Topology) UpdateLink(from, to string, cost float64 /* seq int64*/) {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	if t.edges[from] == nil {
		t.edges[from] = make(map[string]float64)
	}

	if t.edges[to] == nil {
		t.edges[to] = make(map[string]float64)
	}

	// 无向图，更新两条边
	t.edges[from][to] = cost
	t.edges[to][from] = cost

	log.Printf("Topology: Updated link %s-%s cost=%d", from, to, cost)
}

func (t *Topology) RemoveLink(from, to string) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	delete(t.edges[from], to)
	delete(t.edges[to], from)
}

func (t *Topology) GetCost(from, to string) (float64, bool) {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	cost, exists := t.edges[from][to]
	return cost, exists
}

// 获取一个节点的邻接矩阵
func (t *Topology) GetNeighbors(nodeName string) map[string]float64 {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	neighbors := make(map[string]float64)
	for neighbor, cost := range t.edges[nodeName] {
		neighbors[neighbor] = cost
	}
	return neighbors
}

func (t *Topology) GetAllNodes() []*NodeInfo {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	nodes := make([]*NodeInfo, 0, len(t.nodes))
	for _, node := range t.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (t *Topology) GetAdjacencyList() map[string]map[string]float64 {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	adj := make(map[string]map[string]float64)
	for from, neighbors := range t.edges {
		adj[from] = make(map[string]float64)
		for to, cost := range neighbors {
			adj[from][to] = cost
		}
	}

	return adj
}

func (t *Topology) String() string {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	result := "Topology:\n"
	result += "Nodes:\n"
	for name, node := range t.nodes {
		result += fmt.Sprintf(" %s: %s:%d [%s]\n", name, node.IP, node.Port, node.Status)
	}

	result += "Links:\n"
	visited := make(map[string]bool)
	for from, neighbors := range t.edges {
		for to, cost := range neighbors {
			edgeID := makeEdgeID(from, to)
			if !visited[edgeID] {
				result += fmt.Sprintf("  %s-%s: cost=%d seq=%d\n", from, to, cost)
				visited[edgeID] = true
			}
		}
	}

	return result
}

func makeEdgeID(from, to string) string {
	if from < to {
		return from + "-" + to
	}
	return to + "-" + from
}

// containsNode 检查 edgeID 是否包含某个节点
func containsNode(edgeID, nodeName string) bool {
	return edgeID == nodeName+"-" || edgeID == "-"+nodeName ||
		len(edgeID) > len(nodeName) &&
			(edgeID[:len(nodeName)+1] == nodeName+"-" ||
				edgeID[len(edgeID)-len(nodeName)-1:] == "-"+nodeName)
}
