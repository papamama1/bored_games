package controllers

import (
	"code.google.com/p/go.net/websocket"
	"cs309/project/app/chat"
	"cs309/project/app/game"
	"cs309/project/app/models"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/revel/revel"
	"strings"
)

type Social struct {
	App
}

type FriendListResult struct {
	APIResult
	Friends []FriendEntity
}

type FriendEntity struct {
	Id        int
	Name      string
	FirstName string
	LastName  string
	Signature string
	Status    int
}

// Get friend list of current logged in user
// -1 - Not loggedin
//  0 - Successful, Friends contails the list of friends
func (c Social) GetFriendList() revel.Result {
	if !c.isLoggedIn() {
		return c.RenderJson(APIResult{-1})
	}

	user := c.getUserById(c.Session["user.Id"])

	friends := c.getFriends()

	var result FriendListResult
	result.Friends = []FriendEntity{}

	for _, f := range friends {
		var entity FriendEntity

		if f.UserA == user.Id {
			entity.Id = f.UserB
		} else {
			entity.Id = f.UserA
		}

		user := c.getUserById(entity.Id)
		entity.FirstName = user.FirstName
		entity.LastName = user.LastName
		entity.Name = user.Name
		entity.Signature = "Test signature"

		entity.Status = 2

		result.Friends = append(result.Friends, entity)
	}

	return c.RenderJson(result)
}

// Add user with E-Mail friendId as a friend
// -1 - Not logged in
//  0 - Success
//  1 - friendId does not exist
//  2 - Already friends
//  3 - Adding self as friend
//  4 - Friend is not online
func (c Social) AddFriend(friendId string) revel.Result {
	if !c.isLoggedIn() {
		return c.RenderJson(APIResult{-1})
	}

	var friend *models.User

	if strings.Contains(friendId, "@") { // user typed an E-Mail
		friend = c.getUserByEmail(friendId)
		if friend == nil {
			return c.RenderJson(APIResult{1})
		}
	} else { // user typed player name
		friend = c.getUserByName(friendId)
		if friend == nil {
			return c.RenderJson(APIResult{1})
		}
	}

	user := c.getUserById(c.Session["user.Id"])

	if user.Email == friend.Email {
		return c.RenderJson(APIResult{3})
	}

	count, err := c.Txn.SelectInt("select count(*) from Friendship where (UserA = ? and UserB = ?) or (UserA = ? and UserB = ?)", user.Id, friend.Id, friend.Id, user.Id)
	if err != nil {
		panic(err)
	}

	if count > 0 {
		return c.RenderJson(APIResult{2})
	}

	if status := chat.GetStatus(friend.Id); status == -1 {
		return c.RenderJson(APIResult{4})
	}

	// set redis entry to keep track of the request. FriendRequest:FromId:ToId
	// Request lives in redis for 10 minutes
	if _, err := c.Redis.Do("SET", fmt.Sprintf("FriendRequest:%d:%d", user.Id, friend.Id), 1, "EX", 600); err != nil {
		panic(err)
	}
	err = chat.SendEvent(friend.Id, &chat.Event{chat.FRIEND_REQUEST, user.Id, user.GetTagName()})
	if err != nil {
		panic(err)
	}

	return c.RenderJson(APIResult{0})
}

// Confirm friendship
// -1 - not logged in
//  0 - Success
//  1 - Invitation NX
func (c Social) AcceptFriendRequest(id int) revel.Result {
	if !c.isLoggedIn() {
		return c.RenderJson(APIResult{-1})
	}

	user := c.getUserById(c.Session["user.Id"])

	exists, err := redis.Bool(c.Redis.Do("DEL", fmt.Sprintf("FriendRequest:%d:%d", id, user.Id)))
	if err != nil {
		panic(err)
	}

	if !exists {
		return c.RenderJson(APIResult{1})
	}

	friendShip := models.Friendship{id, user.Id}
	c.Txn.Insert(&friendShip)

	if status := chat.GetStatus(id); status > -1 {
		err := chat.SendEvent(id, &chat.Event{chat.MESSAGE, 0, "Refresh"})
		if err != nil {
			panic(err)
		}
	}

	return c.RenderJson(APIResult{0})
}

// Delete friend
// -1 - not logged in
//  0 - Success
//  1 - not friend
func (c Social) RemoveFriend(friendId int) revel.Result {
	if !c.isLoggedIn() {
		return c.RenderJson(APIResult{-1})
	}

	user := c.getUserById(c.Session["user.Id"])

	result, err := c.Txn.Exec("delete from Friendship where (UserA = ? and UserB = ?) or (UserA = ? and UserB = ?)", user.Id, friendId, friendId, user.Id)
	if err != nil {
		panic(err)
	}

	if rows, _ := result.RowsAffected(); rows != 1 {
		return c.RenderJson(APIResult{1})
	}

	if status := chat.GetStatus(friendId); status > -1 {
		err := chat.SendEvent(friendId, &chat.Event{chat.MESSAGE, 0, "Refresh"})
		if err != nil {
			panic(err)
		}
	}

	return c.RenderJson(APIResult{0})
}

// Invite a user to join a game
// -1 - not logged in
//  0 - success
//  1 - user not exist
//  2 - user busy
//  3 - inviting self
//  4 - not online
//  5 - room nx
func (c Social) GameInvite(roomId, friendId string) revel.Result {
	type RoomInvitation struct {
		Name, Game, RoomId, RoomPassword string
	}

	if !c.isLoggedIn() {
		return c.RenderJson(APIResult{-1})
	}

	var friend *models.User

	if strings.Contains(friendId, "@") { // user typed an E-Mail
		friend = c.getUserByEmail(friendId)
		if friend == nil {
			return c.RenderJson(APIResult{1})
		}
	} else { // user typed player name
		friend = c.getUserByName(friendId)
		if friend == nil {
			return c.RenderJson(APIResult{1})
		}
	}

	user := c.getUserById(c.Session["user.Id"])

	if user.Email == friend.Email {
		return c.RenderJson(APIResult{3})
	}

	status := chat.GetStatus(friend.Id)
	if status == -1 {
		return c.RenderJson(APIResult{4})
	} else if status == 1 {
		return c.RenderJson(APIResult{2})
	}

	room, err := game.GetRoom(roomId)
	if err != nil {
		return c.RenderJson(APIResult{5})
	}

	game := c.getGameById(room.GameId)

	roomParam, err := json.Marshal(&RoomInvitation{user.GetTagName(), game.Name, roomId, room.Password})
	if err != nil {
		panic(err)
	}
	err = chat.SendEvent(friend.Id, &chat.Event{chat.GAME_INVITATION, user.Id, string(roomParam)})
	if err != nil {
		panic(err)
	}

	return c.RenderJson(APIResult{0})
}

func (c Social) Feed(ws *websocket.Conn) revel.Result {
	type NewMessage struct {
		To      int // 0 means internal message
		Message string
	}

	if !c.isLoggedIn() {
		return nil
	}

	user := c.getUserById(c.Session["user.Id"])

	event := &chat.Event{chat.COME_ONLINE, user.Id, ""}
	c.sendToFriends(event) // tell everybody I'm online
	sub := chat.Online(user.Id)

	incomingMessage := make(chan *NewMessage)

	go func() {
		var msg *NewMessage

		for {
			msg = new(NewMessage)

			err := websocket.JSON.Receive(ws, msg)
			if err == nil {
				incomingMessage <- msg
			} else {
				close(incomingMessage)
				return
			}
		}
	}()

	for {
		select {
		case event, ok := <-sub.NewEvents:
			if ok {
				websocket.JSON.Send(ws, event)
			} else {
				return nil
			}
		case msg, ok := <-incomingMessage:
			if ok {
				if msg.To == 0 {
					if msg.Message == "GetOnlineStatus" {
						c.Txn.Rollback()
						t, err := DbMap.Begin()
						if err != nil {
							panic(err)
						}
						c.Txn = t
						friends := c.getFriends()
						for _, f := range friends {
							var id int

							if f.UserA == user.Id {
								id = f.UserB
							} else {
								id = f.UserA
							}

							if status := chat.GetStatus(id); status > -1 {
								err := websocket.JSON.Send(ws, &chat.Event{status + 1, id, ""}) // totally hack about this status code...
								if err != nil {
									panic(err)
								}
							}
						}
					}
				} else {
					if status := chat.GetStatus(msg.To); status > -1 {
						err := chat.SendEvent(msg.To, &chat.Event{chat.MESSAGE, user.Id, msg.Message})
						if err != nil {
							panic(err)
						}
					}
				}
			} else {
				err := chat.Offline(user.Id)
				if err != nil {
					panic(err)
				}
				c.sendToFriends(&chat.Event{chat.COME_OFFLINE, user.Id, ""})
				return nil
			}
		}
	}

	return nil
}
