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
	// 构建 Dijkstra 图
	distance := make(map[string]float64)
	nextHop := make(map[string]string) // 直接记录从源节点出发的下一跳

	// 初始化所有节点距离为无穷大
	for _, node := range topology.GetAllNodes() {
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

	visited := make(map[string]bool)
	neighbor_cache := make(map[string]map[string]float64)

	for !pq.Empty() {
		item, _ := pq.Dequeue()
		currentNodeID := item.(*Item).nodeID
		currentCost := item.(*Item).priority

		// 如果已访问过,跳过
		if visited[currentNodeID] {
			continue
		}
		visited[currentNodeID] = true

		neighbors, ok := neighbor_cache[currentNodeID]
		if !ok {
			neighbors = topology.GetNeighbors(currentNodeID)
			neighbor_cache[currentNodeID] = neighbors
		}

		for neighborID, linkCost := range neighbors {
			newCost := currentCost + linkCost
			if newCost < distance[neighborID] {
				distance[neighborID] = newCost

				// 记录下一跳：如果当前节点是源节点，下一跳就是邻居节点
				// 否则，继承当前节点的下一跳
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

		// 添加到路由表
		newRoute := &Route{
			Destination: destNodeID,
			NextHop:     nextHop[destNodeID],
			Cost:        dist,
		}
		routeTable.AddRoute(newRoute)
	}

	return routeTable, nil
}
