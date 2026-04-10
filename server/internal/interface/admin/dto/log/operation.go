package log

type OperationQueryReq struct {
	Current   int    `form:"current" binding:"min=1"`
	Size      int    `form:"size" binding:"min=1,max=100"`
	UserID    uint   `form:"userId"`
	Action    string `form:"action"`
	StartDate string `form:"startDate"`
	EndDate   string `form:"endDate"`
}

func (r *OperationQueryReq) Normalize() {
	if r.Current < 1 {
		r.Current = 1
	}
	if r.Size < 1 {
		r.Size = 10
	}
	if r.Size > 100 {
		r.Size = 100
	}
}

func (r *OperationQueryReq) Offset() int {
	return (r.Current - 1) * r.Size
}
