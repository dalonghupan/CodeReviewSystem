package util

// 分页约束常量（与 proto Pagination 注释一致）
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// NormalizePage 规范化分页参数：页码从1开始，pageSize 默认20、上限100
func NormalizePage(page, pageSize uint32) (uint32, uint32) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}

// PageOffset 计算 SQL OFFSET 值
func PageOffset(page, pageSize uint32) uint32 {
	page, pageSize = NormalizePage(page, pageSize)
	return (page - 1) * pageSize
}

// TotalPages 计算总页数
func TotalPages(total uint32, pageSize uint32) uint32 {
	if pageSize == 0 {
		pageSize = DefaultPageSize
	}
	if total == 0 {
		return 0
	}
	return (total + pageSize - 1) / pageSize
}
