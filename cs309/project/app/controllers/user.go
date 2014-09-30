package controllers

import (
	"code.google.com/p/go.crypto/bcrypt"
	"cs309/project/app/models"
	"cs309/project/app/routes"
	"fmt"
	"github.com/nfnt/resize"
	"github.com/revel/revel"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
)

type User struct {
	App
}

type LoginResult struct {
	APIResult
	Id        int
	Name      string
	FirstName string
	LastName  string
}

type ProfileResult struct {
	APIResult
	Id       int
	Name     string
	IsFriend bool
	Games    []ProfileGameResult
}

type ProfileGameResult struct {
	PlayerId int
	Result   int
	Name     string
}

// Login handler
// 0 - Successful
// 1 - User not found
// 2 - Password incorrect
// 3 - Validation error
func (c User) DoLogin(Email, Password string) revel.Result {
	Email = strings.TrimSpace(Email)
	Password = strings.TrimSpace(Password)

	c.Validation.Required(Email)
	c.Validation.Required(Password)

	if c.Validation.HasErrors() {
		return c.RenderJson(APIResult{3})
	}

	user := c.getUserByEmail(Email)
	if user == nil {
		return c.RenderJson(APIResult{1})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(Password)); err != nil {
		return c.RenderJson(APIResult{2})
	}

	c.Session["user.Id"] = strconv.Itoa(user.Id)
	return c.RenderJson(LoginResult{APIResult{0}, user.Id, user.Name, user.FirstName, user.LastName})

}

// Register user
// 0 - Success
// 1 - Email exists
// 2 - Name exists
// 3 - password doesn't match
// 4 - other
func (c User) DoRegister(user models.User, retypePassword string) revel.Result {
	user.Email = strings.TrimSpace(user.Email)
	user.Name = strings.TrimSpace(user.Name)
	user.FirstName = strings.TrimSpace(user.FirstName)
	user.LastName = strings.TrimSpace(user.LastName)

	c.Validation.Required(retypePassword)
	if retypePassword != user.Password {
		return c.RenderJson(APIResult{3})
	}
	user.Validate(c.Validation)

	// Check to see if the E-Mail already exists
	if u := c.getUserByEmail(user.Email); u != nil {
		return c.RenderJson(APIResult{1})
	}

	count, err := c.Txn.SelectInt("select count(*) from User where Name = ?", user.Name)
	if err != nil {
		panic(err)
	}
	if count != 0 {
		return c.RenderJson(APIResult{2})
	}

	if c.Validation.HasErrors() {
		return c.RenderJson(APIResult{4})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	user.Password = string(hashedPassword)
	err = c.Txn.Insert(&user)
	if err != nil {
		panic(err)
	}

	c.Session["user.Id"] = strconv.Itoa(user.Id)
	return c.RenderJson(APIResult{0})
}

func (c User) Logout() revel.Result {
	for key := range c.Session {
		delete(c.Session, key)
	}
	return c.Redirect(routes.Home.Index())
}

// Check to see if the user is logged in
// -1 - Not logged in
//  0 - Logged in, with user information returned
func (c User) IsLoggedIn() revel.Result {
	if c.isLoggedIn() {
		user := c.getUserById(c.Session["user.Id"])
		return c.RenderJson(LoginResult{APIResult{0}, user.Id, user.Name, user.FirstName, user.LastName})
	} else {
		return c.RenderJson(APIResult{-1})
	}
}

func (c User) Avatar(userId int) revel.Result {
	file, err := os.Open(fmt.Sprintf("%s/avatars/%d.png", revel.BasePath, userId))
	if err != nil {
		file, err = os.Open(revel.BasePath + "/avatars/default.png")
		if err != nil {
			panic(err)
		}
	}

	return c.RenderFile(file, revel.Inline)
}

// Get the profile of a user
//  0 - success
//  1 - person does not exist
func (c User) GetUserProfile(userName string) revel.Result {
	user := c.getUserById(c.Session["user.Id"])
	person := c.getUserByName(userName)
	if person == nil {
		return c.RenderJson(APIResult{1})
	}

	var result ProfileResult

	count, err := c.Txn.SelectInt("select count(*) from Friendship where (UserA = ? and UserB = ?) or (UserA = ? and UserB = ?)", user.Id, person.Id, person.Id, user.Id)
	if err != nil {
		panic(err)
	}

	_, err = c.Txn.Select(&result.Games, "select GameResult.PlayerId, GameResult.Result, Game.Name from GameResult, Game where PlayerId = ? and Game.Id = GameResult.GameId", person.Id)
	if err != nil {
		panic(err)
	}

	result.APIResult = APIResult{0}
	result.Id = person.Id
	result.Name = person.Name
	result.IsFriend = count > 0

	return c.RenderJson(result)
}

// Update user settings
// This is not an Ajax call
func (c User) UpdateSettings(FirstName, LastName, Name, Password string, Avatar io.Reader) revel.Result {
	if !c.isLoggedIn() {
		return c.Redirect(Home.Index)
	}

	FirstName = strings.TrimSpace(FirstName)
	LastName = strings.TrimSpace(LastName)
	Name = strings.TrimSpace(Name)
	Password = strings.TrimSpace(Password)

	user := c.getUserById(c.Session["user.Id"])

	count, err := c.Txn.SelectInt("select count(*) from User where Name = ?", Name)
	if err != nil {
		panic(err)
	}
	if count != 0 && user.Name != Name {
		return c.Redirect("/#/settings")
	}

	user.FirstName = FirstName
	user.LastName = LastName
	user.Name = Name

	if Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(Password), bcrypt.DefaultCost)
		if err != nil {
			panic(err)
		}
		user.Password = string(hashedPassword)
	}
	count, err = c.Txn.Update(user)
	if err != nil {
		panic(err)
	}

	if _, ok := c.Params.Files["Avatar"]; ok { // we have a new avatar
		img, _, err := image.Decode(Avatar)
		if err != nil { // unrecognized format
			return c.Redirect("/#/settings")
		}

		img = resize.Thumbnail(320, 320, img, resize.Lanczos3)

		file, err := os.Create(fmt.Sprintf("%s/avatars/%d.png", revel.BasePath, user.Id))
		if err != nil {
			panic(err)
		}

		defer file.Close()

		err = png.Encode(file, img)
		if err != nil {
			panic(err)
		}
	}

	return c.Redirect(Home.Index)
}

// Check if the given player name already exists
//  0 - No
//  1 - Yes
// -1 - Not logged in
func (c User) NameExists(Name string) revel.Result {
	if !c.isLoggedIn() {
		return c.RenderJson(APIResult{-1})
	}

	Name = strings.TrimSpace(Name)

	count, err := c.Txn.SelectInt("select count(*) from User where Name = ?", Name)
	if err != nil {
		panic(err)
	}
	if count != 0 {
		return c.RenderJson(APIResult{1})
	}

	return c.RenderJson(APIResult{0})
}
