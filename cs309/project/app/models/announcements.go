package models

import (
	"fmt"
	"github.com/revel/revel"
)

type Announcement struct {
	Id    int
	Title string
	Body  string
}

func (e *Announcement) String() string {
	return fmt.Sprintf("Announcement(%s)", e.Title)
}

func (e *Announcement) Validate(v *revel.Validation) {
	v.Check(e.Title,
		revel.ValidRequired(),
		revel.ValidMaxSize(100),
	)
	v.Check(e.Body,
		revel.ValidRequired(),
	)
}
