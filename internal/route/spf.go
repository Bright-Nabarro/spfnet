package route

import (
	"math"

	"github.com/emirpasic/gods/queues/priorityqueue"
	"github.com/emirpasic/gods/utils"
)

// TODO: node增加版本，相同版本直接使用缓存
// SPFCalculator SPF 路径计算器
type SPFCalculator struct {
	// 可以在这里存储一些配置或缓存
	// 比如是否启用路径压缩、是否缓存中间结果等
}

// NewSPFCalculator 创建 SPF 计算器
func NewSPFCalculator() *SPFCalculator {
	return &SPFCalculator{}
}

// ComputeRoutesWithNodes 计算路由（带节点列表）
// 参数：
//   - sourceNode: 源节点 ID
//   - nodes: 所有节点的 ID 列表
//   - adjacencyList: 邻接表
//
// 返回：
//   - *RouteTable: 生成的路由表
//   - error: 如果计算失败返回错误
func (calc *SPFCalculator) ComputeRoutes(
	sourceNodeID string,
	topology *Topology,
) (*RouteTable, error) {

	//获取所有节点，用于初始化 map 容量
	allNodes := topology.GetAllNodes()
	nodeCount := len(allNodes)

	// 构建 Dijkstra 图
	distance := make(map[string]float64, nodeCount)
	nextHop := make(map[string]string, nodeCount)
	previous := make(map[string]string, nodeCount)

	// 初始化所有节点距离为无穷大
	for _, node := range allNodes {
		distance[node.ID] = math.Inf(1)
	}
	distance[sourceNodeID] = 0

	type Item struct {
		nodeID   string
		priority float64
	}

	pq := priorityqueue.NewWith(func(a, b interface{}) int {
		itemA := a.(*Item)
		itemB := b.(*Item)
		return utils.Float64Comparator(itemA.priority, itemB.priority)
	})
	pq.Enqueue(&Item{nodeID: sourceNodeID, priority: 0})

	// 改动：删除了 visited map，直接用距离判断

	// 改动：重命名为驼峰式 neighborCache
	neighborCache := make(map[string]map[string]float64)

	for !pq.Empty() {
		item, _ := pq.Dequeue()
		currentNodeID := item.(*Item).nodeID
		currentCost := item.(*Item).priority

		// 改动：使用距离判断代替 visited map
		// 如果当前从队列取出的路径成本 > 我们已经记录的最短路径，说明是过期数据，跳过
		if currentCost > distance[currentNodeID] {
			continue
		}

		// 检查缓存
		neighbors, ok := neighborCache[currentNodeID]
		if !ok {
			neighbors = topology.GetNeighbors(currentNodeID)
			neighborCache[currentNodeID] = neighbors
		}

		for neighborID, linkCost := range neighbors {
			newCost := currentCost + linkCost

			if newCost < distance[neighborID] {
				distance[neighborID] = newCost
				previous[neighborID] = currentNodeID // 记录前驱节点

				// 记录下一跳
				if currentNodeID == sourceNodeID {
					nextHop[neighborID] = neighborID
				} else {
					nextHop[neighborID] = nextHop[currentNodeID]
				}

				pq.Enqueue(&Item{nodeID: neighborID, priority: newCost})
			}
		}
	}

	// 收集结果到路由表
	routeTable := NewRouteTable(sourceNodeID)

	for destNodeID, dist := range distance {
		// 跳过源节点自己
		if destNodeID == sourceNodeID {
			continue
		}

		// 跳过不可达的节点
		if math.IsInf(dist, 1) {
			continue
		}

		// 改动：路径重建优化
		// 1. 先追加到 Slice 末尾 (O(1))，避免原来的 Prepend (O(N))
		path := make([]string, 0, 8) // 给个默认小容量避免初期扩容
		for at := destNodeID; at != ""; at = previous[at] {
			path = append(path, at)
			if at == sourceNodeID {
				break
			}
		}

		// 2. 反转 Slice
		for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
			path[i], path[j] = path[j], path[i]
		}

		// 添加到路由表
		newRoute := &Route{
			Destination: destNodeID,
			NextHop:     nextHop[destNodeID],
			Cost:        dist,
			Path:        path,
		}
		routeTable.AddRoute(newRoute)
	}

	return routeTable, nil
}
