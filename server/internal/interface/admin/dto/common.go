package dto

type PageQuery struct {
	Current int `form:"current" json:"current"`
	Size    int `form:"size" json:"size"`
}

func (p *PageQuery) Offset() int {
	if p.Current <= 0 {
		p.Current = 1
	}
	if p.Size <= 0 {
		p.Size = 10
	}
	return (p.Current - 1) * p.Size
}

func (p *PageQuery) Limit() int {
	if p.Size <= 0 {
		p.Size = 10
	}
	return p.Size
}
