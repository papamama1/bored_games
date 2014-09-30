package models

import (
	"fmt"
	"github.com/coopernurse/gorp"
)

const (
	TIE = iota
	WIN
	LOSE
	DISCONNECT
)

type GameResult struct {
	RoomId   string
	PlayerId int
	GameId   int
	Result   int

	Player *User
}

func (e *GameResult) String() string {
	return fmt.Sprintf("GameResult(%s:%s)", e.Player.Name, e.Result)
}

func (b *GameResult) PreInsert(_ gorp.SqlExecutor) error {
	if b.Player != nil {
		b.PlayerId = b.Player.Id
	}
	return nil
}

func (b *GameResult) PostGet(exe gorp.SqlExecutor) error {
	var (
		obj interface{}
		err error
	)

	obj, err = exe.Get(User{}, b.PlayerId)
	if err != nil {
		return fmt.Errorf("Error loading a game's player (%d): %s", b.PlayerId, err)
	}
	b.Player = obj.(*User)

	return nil
}
