package grm

//Page Pagination object
type Page struct {
	PageNo     int
	PageSize   int
	TotalCount int
	PageCount  int
	FirstPage  bool
	HasPrev    bool
	HasNext    bool
	LastPage   bool
}

//NewPage Create Page object
func NewPage() *Page {
	page := Page{}
	page.PageNo = 1
	page.PageSize = 20
	return &page
}

//setTotalCount Set the total number of bars, calculate other values
func (page *Page) setTotalCount(total int) {
	page.TotalCount = total
	page.PageCount = (page.TotalCount + page.PageSize - 1) / page.PageSize
	if page.PageNo >= page.PageCount {
		page.LastPage = true
	} else {
		page.HasNext = true
	}
	if page.PageNo > 1 {
		page.HasPrev = true
	} else {
		page.FirstPage = true
	}
}
