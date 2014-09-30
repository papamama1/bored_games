package models

import (
	"fmt"
	"github.com/revel/revel"
)

type User struct {
	Id        int
	Email     string
	Name      string
	FirstName string
	LastName  string
	Password  string
	IsAdmin   bool
}

func (u *User) String() string {
	return fmt.Sprintf("User(%s:%s)", u.Email, u.Name)
}

func (u *User) GetTagName() string {
	return fmt.Sprintf("%s \"%s\" %s", u.FirstName, u.Name, u.LastName)
}

func (user *User) Validate(v *revel.Validation) {
	v.Check(user.Email,
		revel.ValidRequired(),
		revel.ValidEmail(),
	)

	v.Check(user.Name,
		revel.ValidRequired(),
		revel.ValidMaxSize(100),
	)

	v.Check(user.Name,
		revel.ValidRequired(),
		revel.ValidMaxSize(20),
	)

	v.Check(user.FirstName,
		revel.ValidRequired(),
		revel.ValidMaxSize(50),
	)

	v.Check(user.LastName,
		revel.ValidRequired(),
		revel.ValidMaxSize(50),
	)
}
