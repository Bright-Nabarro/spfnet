package route

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/serf/serf"
)

// TopologySync 负责处理拓扑同步和事件传播
type TopologySync struct {
	node     *Node
	topology *Topology

	// 路由计算回调
	onTopologyChange func()

	// 全量同步配置
	syncInterval time.Duration // 全量同步间隔
	syncEnabled  bool          // 是否启用周期性同步

	mtx sync.RWMutex
}

// NewTopologySync 创建拓扑同步管理器
func NewTopologySync(node *Node, topology *Topology) *TopologySync {
	return &TopologySync{
		node:         node,
		topology:     topology,
		syncInterval: 30 * time.Second, // 默认30秒全量同步一次
		syncEnabled:  true,             // 默认启用同步
	}
}

// SetTopologyChangeCallback 设置拓扑变化时的回调函数
func (ts *TopologySync) SetTopologyChangeCallback(callback func()) {
	ts.mtx.Lock()
	defer ts.mtx.Unlock()
	ts.onTopologyChange = callback
}

// SetSyncInterval 设置全量同步间隔
func (ts *TopologySync) SetSyncInterval(interval time.Duration) {
	ts.mtx.Lock()
	defer ts.mtx.Unlock()
	ts.syncInterval = interval
}

// GetSyncInterval 获取当前全量同步间隔
func (ts *TopologySync) GetSyncInterval() time.Duration {
	ts.mtx.RLock()
	defer ts.mtx.RUnlock()
	return ts.syncInterval
}

// EnableSync 启用或禁用周期性同步
func (ts *TopologySync) EnableSync(enabled bool) {
	ts.mtx.Lock()
	defer ts.mtx.Unlock()
	ts.syncEnabled = enabled
	if enabled {
		log.Printf("[%s] Periodic topology sync enabled", ts.node.ID)
	} else {
		log.Printf("[%s] Periodic topology sync disabled", ts.node.ID)
	}
}

// IsSyncEnabled 检查周期性同步是否启用
func (ts *TopologySync) IsSyncEnabled() bool {
	ts.mtx.RLock()
	defer ts.mtx.RUnlock()
	return ts.syncEnabled
}

// Start 启动拓扑同步，监听 Serf 事件
func (ts *TopologySync) Start() {
	ts.node.wg.Add(2)
	go ts.eventLoop()
	go ts.periodicSyncLoop()
}

// eventLoop 处理 Serf 事件
func (ts *TopologySync) eventLoop() {
	defer ts.node.wg.Done()

	for {
		select {
		case <-ts.node.ctx.Done():
			return
		case event := <-ts.node.eventCh:
			ts.handleEvent(event)
		}
	}
}

// handleEvent 处理不同类型的 Serf 事件
func (ts *TopologySync) handleEvent(event serf.Event) {
	switch e := event.(type) {
	case serf.MemberEvent:
		ts.handleMemberEvent(e)
	case serf.UserEvent:
		ts.handleUserEvent(e)
	default:
		log.Printf("Unknown event type: %T", event)
	}
}

// handleMemberEvent 处理成员变化事件（节点加入/离开/失败）
func (ts *TopologySync) handleMemberEvent(event serf.MemberEvent) {
	for _, member := range event.Members {
		switch event.EventType() {
		case serf.EventMemberJoin:
			ts.handleNodeJoin(member)
		case serf.EventMemberLeave, serf.EventMemberFailed:
			ts.handleNodeLeave(member)
		case serf.EventMemberUpdate:
			ts.handleNodeUpdate(member)
		}
	}
}

// handleNodeJoin 处理节点加入事件
func (ts *TopologySync) handleNodeJoin(member serf.Member) {
	port := 0
	if portStr, ok := member.Tags["port"]; ok {
		fmt.Sscanf(portStr, "%d", &port)
	}

	nodeInfo := &NodeInfo{
		ID:     member.Name,
		IP:     member.Tags["ip"],
		Port:   port,
		Status: NodeStatusAlive,
	}

	ts.topology.AddNode(nodeInfo)
	log.Printf("Node joined: %s (%s:%d)", member.Name, member.Tags["ip"], port)

	ts.triggerTopologyChange()
}

// handleNodeLeave 处理节点离开事件
func (ts *TopologySync) handleNodeLeave(member serf.Member) {
	ts.topology.RemoveNode(member.Name)
	log.Printf("Node left: %s", member.Name)

	ts.triggerTopologyChange()
}

// handleNodeUpdate 处理节点更新事件
func (ts *TopologySync) handleNodeUpdate(member serf.Member) {
	log.Printf("Node updated: %s", member.Name)
}

// handleUserEvent 处理用户自定义事件（链路更新）
func (ts *TopologySync) handleUserEvent(event serf.UserEvent) {
	switch event.Name {
	case EventLinkUpdate:
		var linkEvent LinkUpdateEvent
		if err := json.Unmarshal(event.Payload, &linkEvent); err != nil {
			log.Printf("Failed to unmarshal link update event: %v", err)
			return
		}
		ts.handleLinkUpdate(linkEvent)

	case EventTopologySync:
		var syncEvent TopologySyncEvent
		if err := json.Unmarshal(event.Payload, &syncEvent); err != nil {
			log.Printf("Failed to unmarshal topology sync event: %v", err)
			return
		}
		ts.handleTopologySync(syncEvent)
	}
}

// handleLinkUpdate 处理链路更新
func (ts *TopologySync) handleLinkUpdate(event LinkUpdateEvent) {
	switch event.Op {
	case "add", "update":
		ts.topology.UpdateLink(event.From, event.To, event.Cost)
		log.Printf("Link updated: %s-%s cost=%.2f", event.From, event.To, event.Cost)
	case "remove":
		ts.topology.RemoveLink(event.From, event.To)
		log.Printf("Link removed: %s-%s", event.From, event.To)
	}

	ts.triggerTopologyChange()
}

// periodicSyncLoop 定期全量同步拓扑
func (ts *TopologySync) periodicSyncLoop() {
	defer ts.node.wg.Done()

	ts.mtx.RLock()
	interval := ts.syncInterval
	ts.mtx.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ts.node.ctx.Done():
			return
		case <-ticker.C:
			if err := ts.broadcastFullTopology(); err != nil {
				log.Printf("[%s] Failed to broadcast topology: %v", ts.node.ID, err)
			}

			// 检查同步间隔是否被修改，如果修改则重置 ticker
			ts.mtx.RLock()
			currentInterval := ts.syncInterval
			ts.mtx.RUnlock()

			if currentInterval != interval {
				log.Printf("[%s] Sync interval changed from %v to %v, resetting ticker",
					ts.node.ID, interval, currentInterval)
				ticker.Reset(currentInterval)
				interval = currentInterval
			}
		}
	}
}

// broadcastFullTopology 广播完整的拓扑信息
func (ts *TopologySync) broadcastFullTopology() error {
	if ts.node.serf == nil {
		return nil
	}

	// 检查同步是否启用
	ts.mtx.RLock()
	enabled := ts.syncEnabled
	ts.mtx.RUnlock()

	if !enabled {
		return nil
	}

	// 获取所有链路
	adj := ts.topology.GetAdjacencyList()

	var links []TopologyLinkEntry
	visited := make(map[string]bool)

	for from, neighbors := range adj {
		for to, cost := range neighbors {
			// 无向图，避免重复（只记录 from < to 的边）
			edgeID := makeEdgeID(from, to)
			if !visited[edgeID] {
				links = append(links, TopologyLinkEntry{
					From: from,
					To:   to,
					Cost: cost,
				})
				visited[edgeID] = true
			}
		}
	}

	// 构造全量同步事件
	syncEvent := TopologySyncEvent{
		NodeID: ts.node.ID,
		Links:  links,
	}

	payload, err := json.Marshal(syncEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal topology sync event: %w", err)
	}

	// 广播事件
	if err := ts.node.serf.UserEvent(EventTopologySync, payload, false); err != nil {
		return fmt.Errorf("failed to broadcast topology sync: %w", err)
	}

	log.Printf("[%s] Broadcasted full topology: %d links", ts.node.ID, len(links))
	return nil
}

// handleTopologySync 处理接收到的全量拓扑同步
func (ts *TopologySync) handleTopologySync(event TopologySyncEvent) {
	// 跳过自己发送的同步事件
	if event.NodeID == ts.node.ID {
		return
	}

	log.Printf("[%s] Received topology sync from %s: %d links",
		ts.node.ID, event.NodeID, len(event.Links))

	topologyChanged := false

	// 合并接收到的链路信息
	for _, link := range event.Links {
		// 检查本地是否已有该链路
		existingCost, exists := ts.topology.GetCost(link.From, link.To)

		if !exists {
			// 本地没有这条链路，添加它
			ts.topology.UpdateLink(link.From, link.To, link.Cost)
			log.Printf("[%s] Learned new link from %s: %s-%s cost=%.2f",
				ts.node.ID, event.NodeID, link.From, link.To, link.Cost)
			topologyChanged = true
		} else if existingCost != link.Cost {
			// 链路存在但成本不同，使用较小的成本（或其他策略）
			if link.Cost < existingCost {
				ts.topology.UpdateLink(link.From, link.To, link.Cost)
				log.Printf("[%s] Updated link cost from %s: %s-%s cost=%.2f->%.2f",
					ts.node.ID, event.NodeID, link.From, link.To, existingCost, link.Cost)
				topologyChanged = true
			}
		}
	}

	// 如果拓扑发生变化，触发回调
	if topologyChanged {
		ts.triggerTopologyChange()
	}
}

// triggerTopologyChange 触发拓扑变化回调
func (ts *TopologySync) triggerTopologyChange() {
	ts.mtx.RLock()
	callback := ts.onTopologyChange
	ts.mtx.RUnlock()

	if callback != nil {
		callback()
	}
}

// RegisterNode 注册一个节点到拓扑
func (ts *TopologySync) RegisterNode(nodeInfo *NodeInfo) error {
	ts.topology.AddNode(nodeInfo)
	log.Printf("Registered node: %s (%s:%d)", nodeInfo.ID, nodeInfo.IP, nodeInfo.Port)
	return nil
}

// RegisterLink 注册一条链路
func (ts *TopologySync) RegisterLink(from, to string, cost float64) error {
	ts.topology.UpdateLink(from, to, cost)
	log.Printf("Registered link: %s-%s cost=%.2f", from, to, cost)

	// 广播链路更新事件到集群
	if ts.node.serf != nil {
		event := LinkUpdateEvent{
			From: from,
			To:   to,
			Cost: cost,
			Op:   "update",
		}

		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal link event: %w", err)
		}

		if err := ts.node.serf.UserEvent(EventLinkUpdate, payload, false); err != nil {
			return fmt.Errorf("failed to broadcast link update: %w", err)
		}
	}

	ts.triggerTopologyChange()
	return nil
}

// UnregisterNode 注销一个节点
func (ts *TopologySync) UnregisterNode(nodeID string) error {
	ts.topology.RemoveNode(nodeID)
	log.Printf("Unregistered node: %s", nodeID)
	ts.triggerTopologyChange()
	return nil
}

// UnregisterLink 注销一条链路
func (ts *TopologySync) UnregisterLink(from, to string) error {
	ts.topology.RemoveLink(from, to)
	log.Printf("Unregistered link: %s-%s", from, to)

	// 广播链路删除事件
	if ts.node.serf != nil {
		event := LinkUpdateEvent{
			From: from,
			To:   to,
			Op:   "remove",
		}

		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal link event: %w", err)
		}

		if err := ts.node.serf.UserEvent(EventLinkUpdate, payload, false); err != nil {
			return fmt.Errorf("failed to broadcast link removal: %w", err)
		}
	}

	ts.triggerTopologyChange()
	return nil
}

// GetTopology 获取当前拓扑
func (ts *TopologySync) GetTopology() *Topology {
	return ts.topology
}
