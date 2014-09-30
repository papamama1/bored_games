package chat

import (
	"fmt"
)

const (
	ONLINE = iota
	BUSY
)

const (
	MESSAGE = iota
	COME_ONLINE
	COME_BUSY
	COME_OFFLINE
	FRIEND_REQUEST
	GAME_INVITATION
)

type Event struct {
	Type    int // 0 - Message, 1 - Come Online, 2 - Come Busy, 3 - Come Offline
	From    int
	Content string // for chat message
}

type User struct {
	Id        int
	Status    int
	NewEvents chan *Event
}

var (
	alive map[int]*User = make(map[int]*User)
)

func Online(Id int) *User {
	if u, ok := alive[Id]; ok {
		u.Status = ONLINE
	} else {
		alive[Id] = &User{Id, ONLINE, make(chan *Event)}
	}

	return alive[Id]
}

func Busy(Id int) {
	if u, ok := alive[Id]; ok {
		u.Status = BUSY
	} else {
		panic("busy without valid user Id")
	}
}

// Get status of the given Id
// -1 - Offline
func GetStatus(Id int) int {
	if u, ok := alive[Id]; ok {
		return u.Status
	} else {
		return -1
	}
}

func Offline(Id int) error {
	if u, ok := alive[Id]; ok {
		close(u.NewEvents)
		delete(alive, Id)
		return nil
	} else {
		return fmt.Errorf("Could not find user with ID %d", Id)
	}
}

func SendEvent(to int, event *Event) error {
	other, ok := alive[to]
	if !ok {
		return fmt.Errorf("User with id %d is not online", to)
	}

	other.NewEvents <- event

	return nil
}

func GetOnlineUsers() map[int]*User {
	return alive
}
