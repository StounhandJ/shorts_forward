package handlers

import (
	"fmt"
	"strings"

	downloadersService "github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/StounhandJ/shorts_forward/internal/utils"
	telegramUtils "github.com/StounhandJ/shorts_forward/internal/utils/telegram"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

var GlobalCounter = 0

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
	metadataVideo, err := downloader.Download(url)
	if err != nil {
		utils.Log.Error(err)
		results := []telego.InlineQueryResult{}

		return ctx.Bot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
			InlineQueryID: query.ID,
			Results:       results,
			CacheTime:     0,
		})
	}

	GlobalCounter += 1
	if GlobalCounter%10 == 0 {
		utils.Log.Infof("Количество запрошенных роликов %d", GlobalCounter)
	}

	mainInfo := metadataVideo.MainInfo()
	return ctx.Bot().AnswerInlineQuery(ctx, &telego.AnswerInlineQueryParams{
		InlineQueryID: query.ID,
		Results: []telego.InlineQueryResult{
			&telego.InlineQueryResultVideo{
				Type:                  telego.ResultTypeVideo,
				ID:                    metadataVideo.VideoURL[:min(64, len(metadataVideo.VideoURL))],
				Title:                 metadataVideo.Title[:min(200, len(metadataVideo.Title))],
				Caption:               fmt.Sprintf("%s\n%s", metadataVideo.Title[:min(900, len(metadataVideo.Title))], mainInfo),
				VideoURL:              metadataVideo.VideoURL,
				ThumbnailURL:          metadataVideo.ThumbnailURL,
				MimeType:              metadataVideo.MimeType,
				ShowCaptionAboveMedia: true,
				Description:           fmt.Sprintf("%s %s", utils.FormatSecondsToMMSS(metadataVideo.Duration), mainInfo),
				ReplyMarkup:           tu.InlineKeyboard(tu.InlineKeyboardRow(tu.InlineKeyboardButton("Оригинал").WithURL(url))),
			},
		},
		CacheTime: 300,
	})
}

func (h handler) MessageVideo(ctx *th.Context, update telego.Update) error {
	sorryText := "Сори, с этим видео что-то не так и ТГ не смог его скачать🥲\nПростите и не бейте🙏🏿"
	url := telegramUtils.GetMessageText(update)
	// Проверка валидности url
	if !isAllowedShortURL(url) {
		telegramUtils.SendMessage(ctx, false, true, update, "Поддерживается только ссылка на ролик (TikTok, Instagram, YouTube)")

		return nil
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
		telegramUtils.SendMessage(ctx, false, true, update, "Поддерживается только TikTok, Instagram, YouTube")

		return nil
	}

	loadMessage := telegramUtils.SendMessage(ctx, false, true, update, "Загрузка....")

	// Получение данных о видео
	metadataVideo, err := downloader.Download(url)
	if err != nil {
		utils.Log.Error(err)
		telegramUtils.DeleteMessage(ctx, update, loadMessage)
		telegramUtils.SendMessage(ctx, false, true, update, sorryText)

		return nil
	}

	GlobalCounter += 1
	if GlobalCounter%10 == 0 {
		utils.Log.Infof("Количество запрошенных роликов %d", GlobalCounter)
	}

	err = telegramUtils.EditMessage(ctx, update, loadMessage,
		fmt.Sprintf("%s\n%s", metadataVideo.Title[:min(900, len(metadataVideo.Title))], metadataVideo.MainInfo()),
		telegramUtils.InputVideo{
			URL:  metadataVideo.VideoURL,
			Name: metadataVideo.Title[:min(200, len(metadataVideo.Title))],
		},
		tu.InlineKeyboard(tu.InlineKeyboardRow(tu.InlineKeyboardButton("Оригинал").WithURL(url))))
	if err != nil {
		telegramUtils.DeleteMessage(ctx, update, loadMessage)

		telegramUtils.SendMessage(ctx, false, true, update, sorryText)
	}

	return nil
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
