package route

import (
	"fmt"
	"sync"
)

type RouteTable struct {
	mtx        sync.RWMutex
	sourceNode string
	routes     map[string]*Route
}

// NewRouteTable 创建路由表
// 参数：
//   - sourceNode: 本节点的 ID
func NewRouteTable(sourceNode string) *RouteTable {
	return &RouteTable{
		sourceNode: sourceNode,
		routes:     make(map[string]*Route),
	}
}

// GetRoute 获取到某个目的地的路由
// 参数：
//   - destination: 目的节点 ID
//
// 返回：
//   - *Route: 路由信息
//   - error: 如果路由不存在返回错误
func (rt *RouteTable) GetRoute(destination string) (*Route, error) {
	rt.mtx.RLock()
	defer rt.mtx.RUnlock()

	value, ok := rt.routes[destination]
	if !ok {
		return nil, fmt.Errorf("route %s not found", destination)
	}
	return value, nil
}

// GetNextHop 获取到某个目的地的下一跳
// 参数：
//   - destination: 目的节点 ID
//
// 返回：
//   - string: 下一跳节点 ID
//   - error: 如果路由不存在返回错误
func (rt *RouteTable) GetNextHop(destination string) (string, error) {
	rt.mtx.RLock()
	defer rt.mtx.RUnlock()

	route, ok := rt.routes[destination]
	if !ok {
		return "", fmt.Errorf("route %s not found", destination)
	}
	return route.NextHop, nil
}

// AddRoute 添加或更新一条路由
// 参数：
//   - route: 路由信息
func (rt *RouteTable) AddRoute(route *Route) {
	rt.mtx.Lock()
	defer rt.mtx.Unlock()

	rt.routes[route.Destination] = route
}

// RemoveRoute 删除到某个目的地的路由
// 参数：
//   - destination: 目的节点 ID
func (rt *RouteTable) RemoveRoute(destination string) {
	rt.mtx.Lock()
	defer rt.mtx.Unlock()
	delete(rt.routes, destination)
}

// Clear 清空所有路由
func (rt *RouteTable) Clear() {
	rt.mtx.Lock()
	defer rt.mtx.Unlock()
	rt.routes = make(map[string]*Route)
}

// GetAllRoutes 获取所有路由（只读副本）
// 返回：
//   - map[string]*Route: destination -> Route 的映射
func (rt *RouteTable) GetAllRoutes() map[string]*Route {
	rt.mtx.RLock()
	defer rt.mtx.RUnlock()

	// 创建副本
	routesCopy := make(map[string]*Route)
	for dest, route := range rt.routes {
		routesCopy[dest] = route
	}
	return routesCopy
}

// Size 返回路由表中的路由数量
func (rt *RouteTable) Size() int {
	rt.mtx.RLock()
	defer rt.mtx.RUnlock()
	return len(rt.routes)
}

// String 返回路由表的字符串表示（用于调试）
// 格式示例：
//
//	Route Table for Node A:
//	Destination  NextHop  Cost  Path
//	B            B        10    [A B]
//	C            B        30    [A B C]
func (rt *RouteTable) String() string {
	rt.mtx.RLock()
	defer rt.mtx.RUnlock()

	result := fmt.Sprintf("Route Table for Node %s:\n", rt.sourceNode)
	result += fmt.Sprintf("%-12s %-8s %-6s %s\n", "Destination", "NextHop", "Cost", "Path")
	for _, route := range rt.routes {
		result += fmt.Sprintf(
			"%-12s %-8s %-6.2f %v\n",
			route.Destination,
			route.NextHop,
			route.Cost,
			//route.Path,
		)
	}

	return result
}
