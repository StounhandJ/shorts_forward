package handlers

import (
	"context"
	"strings"

	downloadersService "github.com/StounhandJ/shorts_forward/internal/downloaders"
	telegramUtils "github.com/StounhandJ/shorts_forward/internal/utils/telegram"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Стартовое сообщение / Главное меню
func (h handler) StartCommand(ctx *th.Context, update telego.Update) error {
	telegramUtils.SendMessage(ctx, false, false, update, "Это шортс бот by @StounhandJ\ngithub.com/StounhandJ")

	return nil
}

func (h handler) InlineVideo(ctx *th.Context, query telego.InlineQuery) error {
	url := query.Query
	// Проверка валидности url
	if !isAllowedShortURL(url) {
		return ctx.Bot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
			InlineQueryID: query.ID,
			Results:       []telego.InlineQueryResult{},
			CacheTime:     0,
		})
	}

	var downloader downloadersService.IDownloader

	for _, d := range h.downloaders {
		if d.Valid(url) {
			downloader = d

			break
		}
	}

	// Загрузчик не найден
	if downloader == nil {
		return ctx.Bot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
			InlineQueryID: query.ID,
			Results:       []telego.InlineQueryResult{},
			CacheTime:     0,
		})
	}

	// Получение данных о видео
	metadata, err := downloader.Download(url)
	if err != nil {
		results := []telego.InlineQueryResult{}

		return ctx.Bot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
			InlineQueryID: query.ID,
			Results:       results,
			CacheTime:     0,
		})
	}

	// Тестовый вариант с подзагрузкой ролика
	if metadata.VideoReader != nil {
		msg, err := ctx.Bot().SendVideo(context.Background(), &telego.SendVideoParams{
			ChatID: telego.ChatID{ID: 969674918},
			Video:  tu.FileFromReader(*metadata.VideoReader, "example.mp4"),
		})
		if err != nil {
			return err
		}

		return ctx.Bot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
			InlineQueryID: query.ID,
			Results: []telego.InlineQueryResult{
				&telego.InlineQueryResultCachedVideo{
					Type:                  telego.ResultTypeVideo,
					ID:                    msg.Video.FileID,
					Title:                 metadata.Title,
					VideoFileID:           msg.Video.FileID,
					ShowCaptionAboveMedia: true,
				},
			},
			CacheTime: 0,
		})
	}

	return ctx.Bot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
		InlineQueryID: query.ID,
		Results: []telego.InlineQueryResult{
			&telego.InlineQueryResultVideo{
				Type:                  telego.ResultTypeVideo,
				ID:                    metadata.VideoURL[:min(64, len(metadata.VideoURL))],
				Title:                 metadata.Title,
				VideoURL:              metadata.VideoURL,
				ThumbnailURL:          metadata.ThumbnailURL,
				MimeType:              metadata.MimeType,
				ShowCaptionAboveMedia: true,
			},
		},
		CacheTime: 0,
	})
}

// isAllowedShortURL максимально быстрая проверка валидности url на нужные домены
func isAllowedShortURL(s string) bool {
	// Минимальная длина: http://youtube.com/XXXX
	if len(s) < 23 {
		return false
	}

	// Проверка схемы
	switch {
	case strings.HasPrefix(s, "http://"):
		return true
	case strings.HasPrefix(s, "https://"):
		return true
	default:
		return false
	}
}
