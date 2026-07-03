package utils

import "NetyAdmin/internal/domain/entity"

// NormalizePaging 归一化分页参数：page<=0 → 1，size<=0 → entity.DefaultPageSize。
//
// 抽取自 storage/config、storage/record、content/article、task/task 四处
// 复制粘贴的分页兜底逻辑（重构清单 B-OTHER-7）。
// 返回归一化后的 (current, size)。
func NormalizePaging(current, size int) (int, int) {
	if current <= 0 {
		current = 1
	}
	if size <= 0 {
		size = entity.DefaultPageSize
	}
	return current, size
}
