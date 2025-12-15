package route

// Route 表示到某个目的地的路由条目
type Route struct {
	Destination string  // 目的节点 ID
	NextHop     string  // 下一跳节点 ID
	Cost        float64 // 到目的地的总代价
	//Path        []string // 完整路径（从源到目的地的所有节点）
}
