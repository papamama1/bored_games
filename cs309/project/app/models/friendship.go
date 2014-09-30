package models

import (
	"fmt"
)

type Friendship struct {
	UserA int
	UserB int
}

func (u *Friendship) String() string {
	return fmt.Sprintf("Friendship(%s:%s)", u.UserA, u.UserB)
}
