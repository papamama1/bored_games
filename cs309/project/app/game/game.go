package game

import (
	"container/list"
	"cs309/project/app"
	"errors"
)

const (
	SYSTEM = iota
	CHAT
	JOIN
	LEAVE
	GAMEMESSAGE
	REPORT
)

const (
	JOINING = iota
	STARTED
	ENDED
)

type Event struct {
	Type    int
	From    int
	Content string // for chat message
}

type User struct {
	Id       int
	Name     string
	NewEvent chan *Event // outgoing event channel
}

// User entity for user list
type UserEntity struct {
	Id   int
	Name string
}

type Room struct {
	Users *list.List // user that are currently in the room
	//Started           bool        // game has been started
	Incoming          chan *Event // incoming event channel
	Host              int         // current host user id, 0 if no one is in the room
	Name              string
	GameId            int
	Password          string
	Status            int // status of the room
	MasterId          int // current master
	Capacity          int
	DisconnectedUsers []int
}

var (
	rooms          map[string]*Room = make(map[string]*Room)
	ErrNxRoom                       = errors.New("Room does not exist")
	ErrGameStarted                  = errors.New("Game already started")
	ErrNxUser                       = errors.New("User does not exist")
)

func (r *Room) broadcast(event *Event) {
	for e := r.Users.Front(); e != nil; e = e.Next() {
		if event.From != e.Value.(*User).Id {
			e.Value.(*User).NewEvent <- event
		}
	}
}

func (r *Room) StatusToString() string {
	if r.Status == JOINING {
		return "Not Started"
	} else if r.Status == STARTED {
		return "Game Started"
	} else if r.Status == ENDED {
		return "Game Ended"
	}

	return "Unknown"
}

func GetAllRooms() map[string]*Room {
	return rooms
}

func NewRoom(name string, gameId int, password string, capacity int) (string, *Room) {
	room := &Room{list.New(), make(chan *Event), 0, name, gameId, password, JOINING, 0, capacity, nil}
	roomId := app.GenerateUUID()
	rooms[roomId] = room

	go func() { // main room loop
		for {
			if e, ok := <-room.Incoming; ok {
				room.broadcast(e)
			} else {
				return
			}
		}
	}()

	return roomId, room
}

func GetRoom(roomId string) (*Room, error) {
	room, ok := rooms[roomId]
	if !ok {
		return nil, ErrNxRoom
	}

	return room, nil
}

func JoinRoom(userId int, userName, roomId string) (*User, error) {
	room, ok := rooms[roomId]
	if !ok {
		return nil, ErrNxRoom
	}

	if room.Status != JOINING {
		return nil, ErrGameStarted
	}

	user := &User{userId, userName, make(chan *Event)}
	if room.Host == 0 {
		room.Host = userId
	}
	room.Users.PushBack(user)
	room.Incoming <- &Event{JOIN, userId, userName}

	return user, nil
}

func LeaveRoom(userId int, roomId string) error {
	room, ok := rooms[roomId]
	if !ok {
		return ErrNxRoom
	}

	user, e := room.userInRoom(userId)
	if user == nil {
		return ErrNxUser
	}

	close(user.NewEvent) // shut down event feed channel, this will cause the websocket event loop to stop
	room.Users.Remove(e)

	if room.Users.Len() > 0 { // there are stil other player in the room
		room.Incoming <- &Event{LEAVE, userId, ""}
	} else { // there is no one left, shutdown room
		close(room.Incoming)
		delete(rooms, roomId)
	}

	return nil
}

func SendEvent(userId int, roomId string, event *Event) error {
	room, ok := rooms[roomId]
	if !ok {
		return ErrNxRoom
	}

	user, _ := room.userInRoom(userId)
	if user == nil {
		return ErrNxUser
	}

	room.Incoming <- event

	return nil
}

func GetRoomUserList(roomId string) ([]UserEntity, error) {
	room, ok := rooms[roomId]
	if !ok {
		return nil, ErrNxRoom
	}

	var result []UserEntity

	for e := room.Users.Front(); e != nil; e = e.Next() {
		result = append(result, UserEntity{e.Value.(*User).Id, e.Value.(*User).Name})
	}

	return result, nil
}

func (r *Room) userInRoom(userId int) (*User, *list.Element) {
	for e := r.Users.Front(); e != nil; e = e.Next() {
		if e.Value.(*User).Id == userId {
			return e.Value.(*User), e
		}
	}

	return nil, nil
}
