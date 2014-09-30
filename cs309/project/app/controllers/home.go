package controllers

import (
	"cs309/project/app/models"
	"github.com/revel/revel"
	"html/template"
)

type Home struct {
	App
}

func (c Home) Index() revel.Result {
	chatAvatarSrc := template.HTMLAttr(`ng-src="/avatar/{{ f.Id }}"`)
	inboxAvatarSrc := template.HTMLAttr(`ng-src="/avatar/{{ m.From }}"`)
	return c.Render(chatAvatarSrc, inboxAvatarSrc)
}

func (c Home) GetAnnouncements() revel.Result {
	var announcements []models.Announcement
	_, err := c.Txn.Select(&announcements, "select * from Announcement")
	if err != nil {
		panic(err)
	}

	return c.RenderJson(&announcements)
}
