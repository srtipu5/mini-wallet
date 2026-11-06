package dto

// Pagination is the "pagination" object of every list response.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// NewPagination fills in the totals for one page of a list.
func NewPagination(page, pageSize, totalItems int) Pagination {
	return Pagination{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: (totalItems + pageSize - 1) / pageSize,
	}
}
