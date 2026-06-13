package utils

// BuildTree 通用的 O(N) 级别树构建算法，支持节点过滤
// T 是源数据节点类型 (Entity)
// R 是目标树节点类型 (DTO/VO)
// convert 返回转换后的节点和是否保留该节点 (true: 保留, false: 丢弃)
func BuildTree[T any, R any](
	elements []T,
	getParentID func(T) uint,
	getID func(T) uint,
	convert func(node T, children []R) (R, bool),
) []R {
	// 1. 按 ParentID 聚合子节点
	childrenMap := make(map[uint][]T)
	var roots []T
	for _, el := range elements {
		pid := getParentID(el)
		if pid == 0 {
			roots = append(roots, el)
		} else {
			childrenMap[pid] = append(childrenMap[pid], el)
		}
	}

	// 2. 递归构建树（带 visited 集合防止自引用/循环引用导致栈溢出）
	var build func(uint, map[uint]bool) []R
	build = func(parentID uint, visited map[uint]bool) []R {
		children := childrenMap[parentID]
		if len(children) == 0 {
			return nil
		}
		var dtos []R
		for _, child := range children {
			childID := getID(child)
			// 防止自引用或循环引用导致无限递归
			if visited[childID] {
				continue
			}
			visited[childID] = true
			// 递归获取子树
			subChildren := build(childID, visited)
			// 转换为目标节点并决定是否保留
			node, keep := convert(child, subChildren)
			if keep {
				dtos = append(dtos, node)
			}
		}
		return dtos
	}

	// 3. 从根节点开始构建
	var result []R
	rootVisited := make(map[uint]bool)
	for _, root := range roots {
		rootID := getID(root)
		rootVisited[rootID] = true
		subChildren := build(rootID, rootVisited)
		node, keep := convert(root, subChildren)
		if keep {
			result = append(result, node)
		}
	}

	return result
}
