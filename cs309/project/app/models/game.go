package models

import (
	"fmt"
	"github.com/coopernurse/gorp"
	"github.com/revel/revel"
)

type Game struct {
	Id      int
	Name    string
	OwnerId int
	URL     string

	Owner *User
}

func (e *Game) String() string {
	return fmt.Sprintf("Game(%s:%s)", e.Name)
}

func (e *Game) Validate(v *revel.Validation) {
	v.Check(e.Name,
		revel.ValidRequired(),
		revel.ValidMaxSize(100),
	)
	v.Check(e.URL,
		revel.ValidRequired(),
	)
}

func (b *Game) PreInsert(_ gorp.SqlExecutor) error {
	b.OwnerId = b.Owner.Id
	return nil
}

func (b *Game) PostGet(exe gorp.SqlExecutor) error {
	var (
		obj interface{}
		err error
	)

	obj, err = exe.Get(User{}, b.OwnerId)
	if err != nil {
		return fmt.Errorf("Error loading a game's owner (%d): %s", b.OwnerId, err)
	}
	b.Owner = obj.(*User)

	return nil
}
