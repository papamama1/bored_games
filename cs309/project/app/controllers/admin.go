package controllers

import (
	"cs309/project/app/chat"
	"cs309/project/app/game"
	"cs309/project/app/models"
	"database/sql"
	"fmt"
	"github.com/revel/revel"
	"os"
)

type Admin struct {
	App
}

func (c Admin) isAdmin() bool {
	if !c.isLoggedIn() {
		return false
	}
	user := c.getUserById(c.Session["user.Id"])
	if !user.IsAdmin {
		return false
	}
	return true
}

func (c Admin) Index() revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	userCount, err := c.Txn.SelectInt("select count(*) from User")
	if err != nil {
		panic(err)
	}

	gameCount, err := c.Txn.SelectInt("select count(*) from Game")
	if err != nil {
		panic(err)
	}

	online := chat.GetOnlineUsers()
	onlineCount := len(online)

	rooms := game.GetAllRooms()
	roomCount := len(rooms)

	Action := "Index"
	return c.Render(Action, userCount, gameCount, onlineCount, roomCount)
}

func (c Admin) Games() revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	results, err := c.Txn.Select(models.Game{}, "select * from Game order by Name")
	if err != nil {
		panic(err)
	}
	var games []*models.Game
	for _, r := range results {
		b := r.(*models.Game)
		games = append(games, b)
	}

	Action := "Games"
	return c.Render(Action, games)
}

func (c Admin) Announcements() revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	results, err := c.Txn.Select(models.Announcement{}, "select * from Announcement")
	if err != nil {
		panic(err)
	}
	var announcements []*models.Announcement
	for _, r := range results {
		b := r.(*models.Announcement)
		announcements = append(announcements, b)
	}

	Action := "Announcements"
	return c.Render(Action, announcements)
}

func (c Admin) Users() revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	results, err := c.Txn.Select(models.User{}, "select * from User")
	if err != nil {
		panic(err)
	}
	var users []*models.User
	for _, r := range results {
		b := r.(*models.User)
		users = append(users, b)
	}

	Action := "Users"
	return c.Render(Action, users)
}

func (c Admin) Rooms() revel.Result {
	type RoomsResult struct {
		Room     *game.Room
		GameName string
		RoomId   string
	}
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	var result []RoomsResult
	rooms := game.GetAllRooms()
	for id, r := range rooms {
		var g models.Game
		err := c.Txn.SelectOne(&g, "select * from Game where Id = ?", r.GameId)
		if err != nil {
			panic(err)
		}
		result = append(result, RoomsResult{r, g.Name, id})
	}

	Action := "Rooms"
	return c.Render(Action, result)
}

func (c Admin) OnlineUsers() revel.Result {
	type OnlineUserResult struct {
		Room    *chat.User
		TagName string
	}
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	online := chat.GetOnlineUsers()
	var result []OnlineUserResult
	for _, r := range online {
		var g models.User
		err := c.Txn.SelectOne(&g, "select * from User where Id = ?", r.Id)
		if err != nil {
			panic(err)
		}
		result = append(result, OnlineUserResult{r, g.GetTagName()})
	}

	Action := "OnlineUsers"
	return c.Render(Action, result)
}

func (c Admin) AddGame() revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	Action := "Games"
	return c.Render(Action)
}

func (c Admin) AddAnnouncement() revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	Action := "Announcements"
	return c.Render(Action)
}

func (c Admin) DoAddGame(game models.Game, Email string) revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	game.Validate(c.Validation)
	var user models.User
	err := c.Txn.SelectOne(&user, "select * from User where Email = ?", Email)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}
	c.Validation.Required(err == nil).Key("Email").Message("E-Mail does not exist")
	game.Owner = &user

	if c.Validation.HasErrors() {
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect(Admin.AddGame)
	}

	err = c.Txn.Insert(&game)
	if err != nil {
		panic(err)
	}

	c.Flash.Success("Game added successfully")
	return c.Redirect(Admin.Games)
}

func (c Admin) DoAddAnnouncement(announcement models.Announcement) revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	announcement.Validate(c.Validation)

	if c.Validation.HasErrors() {
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect(Admin.AddAnnouncement)
	}

	err := c.Txn.Insert(&announcement)
	if err != nil {
		panic(err)
	}

	c.Flash.Success("Announcement published successfully")
	return c.Redirect(Admin.Announcements)
}

func (c Admin) DeleteGame(gameId int) revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	row, err := c.Txn.Get(models.Game{}, gameId)
	if err != nil {
		panic(err)
	}

	if row == nil {
		c.Flash.Error("Game does not exist")
		return c.Redirect(Admin.Games)
	}

	_, err = c.Txn.Exec("delete from GameResult where GameId = ?", gameId)
	if err != nil {
		panic(err)
	}

	_, err = c.Txn.Delete(row)
	if err != nil {
		panic(err)
	}

	c.Flash.Success("Game deleted successfully")
	return c.Redirect(Admin.Games)
}

func (c Admin) DeleteAnnouncement(announcementId int) revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	row, err := c.Txn.Get(models.Announcement{}, announcementId)
	if err != nil {
		panic(err)
	}

	if row == nil {
		c.Flash.Error("Announcement does not exist")
		return c.Redirect(Admin.Announcements)
	}

	_, err = c.Txn.Delete(row)
	if err != nil {
		panic(err)
	}

	c.Flash.Success("Announcement deleted successfully")
	return c.Redirect(Admin.Announcements)
}

func (c Admin) DeleteUser(userId int) revel.Result {
	if !c.isAdmin() {
		return c.Forbidden("Unauthorized access")
	}

	row, err := c.Txn.Get(models.User{}, userId)
	if err != nil {
		panic(err)
	}

	if row == nil {
		c.Flash.Error("user does not exist")
		return c.Redirect(Admin.Users)
	}

	os.Remove(fmt.Sprintf("%s/avatars/%d.png", revel.BasePath, userId))

	_, err = c.Txn.Exec("delete from GameResult where PlayerId = ?", userId)
	if err != nil {
		panic(err)
	}

	_, err = c.Txn.Delete(row)
	if err != nil {
		panic(err)
	}

	c.Flash.Success("User deleted successfully")
	return c.Redirect(Admin.Users)
}
