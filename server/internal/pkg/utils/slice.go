package utils

// SliceMap 将类型 T 的切片通过映射函数转换为类型 R 的切片
func SliceMap[T any, R any](items []T, mapper func(T) R) []R {
	if items == nil {
		return nil
	}
	result := make([]R, len(items))
	for i, item := range items {
		result[i] = mapper(item)
	}
	return result
}
