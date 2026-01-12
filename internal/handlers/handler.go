package handlers

import (
	downloadersService "github.com/StounhandJ/shorts_forward/internal/downloaders"
	th "github.com/mymmrac/telego/telegohandler"
)

type handler struct {
	downloaders []downloadersService.IDownloader
}

func NewHandler(downloaders []downloadersService.IDownloader) handler {
	return handler{
		downloaders: downloaders,
	}
}

func (h handler) SetupRoutes(bh *th.BotHandler) {
	// Базовые действия
	bh.Handle(h.StartCommand, th.CommandEqual("start"))

	bh.HandleInlineQuery(h.InlineVideo)
}
