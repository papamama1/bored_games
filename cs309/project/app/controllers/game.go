package controllers

import (
	"code.google.com/p/go.net/websocket"
	"cs309/project/app/chat"
	"cs309/project/app/game"
	"cs309/project/app/models"
	"cs309/project/app/routes"
	"encoding/json"
	"github.com/revel/revel"
	"io/ioutil"
	"net/http"
)

type Game struct {
	App
}

type GameEntity struct {
	Id   int
	Name string
	URL  string
}

type GameListResult struct {
	APIResult
	Games []GameEntity
}

type RoomResult struct {
	APIResult
	RoomId            string
	GameName          string
	GameURL           string
	RoomName          string
	PasswordProtected bool
	Capacity          int
}

const (
	TIE = iota
	WIN
)

type GameResult struct {
	Type   int
	Winner int
}

func (c Game) List() revel.Result {
	var games []models.Game
	_, err := c.Txn.Select(&games, "select * from Game")
	if err != nil {
		panic(err)
	}

	var result GameListResult
	result.Games = []GameEntity{}

	for _, g := range games {
		result.Games = append(result.Games, GameEntity{g.Id, g.Name, g.URL})
	}

	return c.RenderJson(result)
}

func (c Game) Create(roomName string, gameId int, password string, capacity int) revel.Result {
	if !c.isLoggedIn() {
		return nil
	}

	var g models.Game
	err := c.Txn.SelectOne(&g, "select * from Game where Id = ?", gameId)
	if err != nil {
		panic(err)
	}

	id, _ := game.NewRoom(roomName, gameId, password, capacity)
	return c.RenderJson(&RoomResult{APIResult{0}, id, g.Name, g.URL, roomName, password != "", capacity})
}

// Proxy the request to the actual URL
// This is required since we need to get around same origin policy
// in order to inject the js object
func (c Game) Proxy(gameId int) revel.Result {
	if !c.isLoggedIn() {
		return nil
	}

	game := c.getGameById(gameId)
	resp, err := http.Get(game.URL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	page, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return c.RenderHtml(string(page))
}

// Get information about an existing room
// 0 - Success
// 1 - Room does not exist
// 2 - Need password
// 3 - Started
// 4 - Over capacity
func (c Game) RoomInfo(roomId, password string) revel.Result {
	if !c.isLoggedIn() {
		return nil
	}

	room, err := game.GetRoom(roomId)
	if err != nil {
		return c.RenderJson(APIResult{1})
	}

	if room.Status == game.STARTED {
		return c.RenderJson(APIResult{3})
	}

	if room.Password != password { // password protected room
		return c.RenderJson(APIResult{2})
	}

	if room.Users.Len() >= room.Capacity {
		return c.RenderJson(APIResult{4})
	}

	game := c.getGameById(room.GameId)

	return c.RenderJson(&RoomResult{APIResult{0}, roomId, game.Name, routes.Game.Proxy(game.Id), room.Name, room.Password != "", room.Capacity})
}

func (c Game) RecentGame() revel.Result {
	if !c.isLoggedIn() {
		return nil
	}
	user := c.getUserById(c.Session["user.Id"])

	var results []struct {
		PlayerId int
		Result   int
		Name     string
	}

	_, err := c.Txn.Select(&results, "select GameResult.PlayerId, GameResult.Result, Game.Name from GameResult, Game where PlayerId = ? and Game.Id = GameResult.GameId limit 10", user.Id)
	if err != nil {
		panic(err)
	}

	return c.RenderJson(&results)
}

func (c Game) GameHistoryByPlayer(playerId int) revel.Result {
	if !c.isLoggedIn() {
		return nil
	}
	var results []struct {
		PlayerId int
		Result   int
		Name     string
	}

	_, err := c.Txn.Select(&results, "select GameResult.PlayerId, GameResult.Result, Game.Name from GameResult, Game where PlayerId = ? and Game.Id = GameResult.GameId", playerId)
	if err != nil {
		panic(err)
	}

	return c.RenderJson(&results)
}

func (c Game) Socket(roomId string, ws *websocket.Conn) revel.Result {
	if !c.isLoggedIn() {
		return nil
	}

	user := c.getUserById(c.Session["user.Id"])
	sub, err := game.JoinRoom(user.Id, user.Name, roomId)
	if err != nil {
		return nil
	}

	chat.Busy(user.Id)
	c.sendToFriends(&chat.Event{chat.COME_BUSY, user.Id, ""})

	incoming := make(chan *game.Event)
	go func() { // Receive() blocks so we have to run the incoming loop in a seprate goroutine
		// and feed the events to the main event loop
		var msg *game.Event

		for {
			msg = new(game.Event)

			err := websocket.JSON.Receive(ws, msg)
			if err == nil {
				incoming <- msg
			} else {
				close(incoming)
				return
			}
		}
	}()

	room, err := game.GetRoom(roomId)
	if err != nil {
		panic(err)
	}

	for {
		select {
		case event, ok := <-incoming:
			if ok {
				if event.Type == game.SYSTEM {
					if event.Content == "GetRoomUserList" {
						users, err := game.GetRoomUserList(roomId)
						if err != nil {
							panic(err)
						}

						for _, u := range users {
							if err = websocket.JSON.Send(ws, &game.Event{game.JOIN, u.Id, u.Name}); err != nil {
								panic(err)
							}
						}
					} else if event.Content == "StartGame" {
						if room.Status != game.JOINING {
							continue
						}

						room.Status = game.STARTED
						room.MasterId = user.Id
						switchMasterEvent := &game.Event{game.SYSTEM, user.Id, "SwitchMaster"}
						if err = game.SendEvent(user.Id, roomId, switchMasterEvent); err != nil {
							panic(err)
						}
						if err = websocket.JSON.Send(ws, switchMasterEvent); err != nil {
							panic(err)
						}

						if err = game.SendEvent(user.Id, roomId, event); err != nil {
							panic(err)
						}
						if err = websocket.JSON.Send(ws, event); err != nil {
							panic(err)
						}
					}
				} else if event.Type == game.REPORT {
					if room.Status != game.STARTED {
						continue
					}
					room.Status = game.ENDED

					var result GameResult
					err = json.Unmarshal([]byte(event.Content), &result)
					if err != nil {
						panic(err)
					}

					users, err := game.GetRoomUserList(roomId)
					if err != nil {
						panic(err)
					}

					for _, u := range users {
						var dbResult models.GameResult
						dbResult.RoomId = roomId
						dbResult.PlayerId = u.Id
						dbResult.GameId = room.GameId
						if result.Type == TIE {
							dbResult.Result = models.TIE
						} else {
							if result.Winner == u.Id {
								dbResult.Result = models.WIN
							} else {
								dbResult.Result = models.LOSE
							}
						}
						err = c.Txn.Insert(&dbResult)
						if err != nil {
							panic(err)
						}
					}

					for _, id := range room.DisconnectedUsers {
						var dbResult models.GameResult
						dbResult.RoomId = roomId
						dbResult.PlayerId = id
						dbResult.GameId = room.GameId
						dbResult.Result = models.DISCONNECT
						err = c.Txn.Insert(&dbResult)
						if err != nil {
							panic(err)
						}
					}

					c.Txn.Commit()
					t, err := DbMap.Begin()
					if err != nil {
						panic(err)
					}
					c.Txn = t

					if err = game.SendEvent(user.Id, roomId, event); err != nil {
						panic(err)
					}
					if err = websocket.JSON.Send(ws, event); err != nil {
						panic(err)
					}
				} else {
					if err = game.SendEvent(user.Id, roomId, event); err != nil {
						panic(err)
					}
				}
			} else {
				if err = game.LeaveRoom(user.Id, roomId); err != nil {
					panic(err)
				}

				// If there are still people in room and game is still going,and I am master. switch master
				if room, err := game.GetRoom(roomId); err == nil && room.Status == game.STARTED {
					if user.Id == room.MasterId {
						switchMasterEvent := &game.Event{game.SYSTEM, room.Users.Front().Value.(*game.User).Id, "SwitchMaster"}
						room.Users.Front().Value.(*game.User).NewEvent <- switchMasterEvent
						room.Incoming <- switchMasterEvent
					}

					room.DisconnectedUsers = append(room.DisconnectedUsers, user.Id)
				}
				chat.Online(user.Id)
				c.sendToFriends(&chat.Event{chat.COME_ONLINE, user.Id, ""})
				return nil
			}
		case event, ok := <-sub.NewEvent:
			if ok {
				if err = websocket.JSON.Send(ws, event); err != nil {
					panic(err)
				}
			} else {
				return nil
			}
		}
	}
}
