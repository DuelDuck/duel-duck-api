package repository

var OperatorsMap = map[string]struct{}{
	"=":      {},
	"<":      {},
	">":      {},
	"<=":     {},
	">=":     {},
	"!=":     {},
	"@>":     {},
	"like":   {},
	"ilike":  {},
	"is":     {},
	"is not": {},
}

type Options struct {
	Pagination Pagination `json:"pagination" query:"pagination"`
	Order      Order      `json:"order" query:"order"`
	Filters    []Filter   `json:"filters" query:"filters"`
}

type Pagination struct {
	PageSize uint64 `json:"page_size" query:"page_size"`
	PageNum  uint64 `json:"page_num" query:"page_num"`
}

func (p Pagination) IsValid() bool {
	return p.PageSize > 0 && p.PageNum > 0
}

type Order struct {
	OrderBy   string `json:"order_by" query:"order_by"`
	OrderType string `json:"order_type" query:"order_type"`
}

const (
	OrderDesc = "desc"
	OrderAsc  = "asc"
)

func (o Order) IsValid() bool {
	return (o.OrderType == OrderDesc || o.OrderType == OrderAsc) && o.OrderBy != ""
}

type Filter struct {
	Column   string `json:"column" query:"column"`
	Operator string `json:"operator" query:"operator"`
	Value    string `json:"value" query:"value"`
	WhereOr  bool   `json:"where_or" query:"where_or"`
}

func (f Filter) isValid() bool {
	if f.Column == "" || f.Operator == "" || f.Value == "" {
		return false
	}

	if _, ok := OperatorsMap[f.Operator]; !ok {
		return false
	}

	return true
}
