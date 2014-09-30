package controllers

// import "github.com/robfig/revel"
import (
	"cs309/project/app/chat"
	"cs309/project/app/models"
	"strconv"
)

type APIResult struct {
	Code int
}

type App struct {
	GorpController
}

func (c App) getUserByEmail(Email string) *models.User {
	var users []models.User
	_, err := c.Txn.Select(&users, "select * from User where Email = ?", Email)
	if err != nil {
		panic(err)
	}
	if len(users) == 0 {
		return nil
	}

	return &users[0]
}

func (c App) getUserByName(Name string) *models.User {
	var users []models.User
	_, err := c.Txn.Select(&users, "select * from User where Name = ?", Name)
	if err != nil {
		panic(err)
	}
	if len(users) == 0 {
		return nil
	}

	return &users[0]
}

// since session are all strings, we convert it in here
func (c App) getUserById(Id interface{}) *models.User {
	switch Id.(type) {
	case string:
		Id, _ = strconv.Atoi(Id.(string))
	case int:
	default:
		panic("Unknown type of Id")
	}

	var users []models.User
	_, err := c.Txn.Select(&users, "select * from User where Id = ?", Id.(int))
	if err != nil {
		panic(err)
	}
	if len(users) == 0 {
		return nil
	}

	return &users[0]
}

func (c App) getGameById(Id int) *models.Game {
	var game models.Game
	err := c.Txn.SelectOne(&game, "select * from Game where Id = ?", Id)
	if err != nil {
		panic(err)
	}

	return &game
}

// Get slice of all friends of current logged in user
// Check if the user is logged in before calling
func (c App) getFriends() []models.Friendship {
	user := c.getUserById(c.Session["user.Id"])

	var friends []models.Friendship
	_, err := c.Txn.Select(&friends, "select * from Friendship where UserA = ? or UserB = ?", user.Id, user.Id)

	if err != nil {
		panic(err)
	}

	return friends
}

func (c App) sendToFriends(e *chat.Event) {
	user := c.getUserById(c.Session["user.Id"])
	friends := c.getFriends()
	for _, f := range friends {
		var id int

		if f.UserA == user.Id {
			id = f.UserB
		} else {
			id = f.UserA
		}

		if status := chat.GetStatus(id); status > -1 {
			err := chat.SendEvent(id, e)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (c App) isLoggedIn() bool {
	_, ok := c.Session["user.Id"]
	return ok
}
