package telegram

import (
	"bytes"
	"errors"
	"io"

	"github.com/StounhandJ/shorts_forward/internal/utils"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

type InputVideo struct {
	URL  string
	Name string
}

func DownloadFile(ctx *th.Context, update telego.Update) (io.Reader, error) {
	if update.Message == nil || update.Message.Document == nil {
		return nil, errors.New("файл не прикреплен")
	}

	file, err := ctx.Bot().GetFile(ctx, &telego.GetFileParams{
		FileID: update.Message.Document.FileID,
	})
	if err != nil {
		return nil, err
	}

	fileData, err := tu.DownloadFile(ctx.Bot().FileDownloadURL(file.FilePath))
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(fileData), nil
}

// Получение ID отправителя сообщения или события
func GetUserID(update telego.Update) int64 {
	if update.Message != nil {
		if !update.Message.From.IsBot {
			return update.Message.From.ID
		} else {
			return update.Message.Chat.ID
		}
	}

	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}

	return 0
}

// Получение ID чата
func GetChatID(update telego.Update) int64 {
	if update.Message != nil {
		return update.Message.Chat.ID
	}

	return 0
}

// Получение данных указанных в кнопке для callback
func GetCallbackData(update telego.Update) string {
	if update.CallbackQuery != nil {
		return update.CallbackQuery.Data
	}

	return ""
}

// Получение текста сообщения
func GetMessageText(update telego.Update) string {
	if update.Message != nil {
		return update.Message.Text
	}

	if update.CallbackQuery != nil {
		return update.CallbackQuery.Message.Message().Text
	}

	return ""
}

// Получение ID текущего сообщения
func GetCurrentMessageID(update telego.Update) int {
	if update.Message != nil && !update.Message.From.IsBot {
		return update.Message.MessageID
	}

	if update.CallbackQuery != nil {
		return update.CallbackQuery.Message.GetMessageID()
	}

	return 0
}

// Отправка сообщения
func SendMessage(ctx *th.Context, isChat, isSendReplay bool, update telego.Update, text string, args ...any) int {
	var meesageParam *telego.SendMessageParams
	var inputFile *InputVideo

	sendChatID := GetUserID(update)
	if isChat {
		sendChatID = GetChatID(update)
	}

	meesageParam = &telego.SendMessageParams{
		ChatID:    tu.ID(sendChatID),
		Text:      text[:min(4096, len(text))],
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
	}

	if isSendReplay {
		meesageParam.ReplyParameters = &telego.ReplyParameters{
			MessageID:                GetCurrentMessageID(update),
			ChatID:                   tu.ID(sendChatID),
			AllowSendingWithoutReply: true,
		}
	}

	for _, v := range args {
		// nolint
		switch v.(type) {
		case telego.ReplyMarkup:
			meesageParam.ReplyMarkup = v.(telego.ReplyMarkup)
		case InputVideo:
			file := v.(InputVideo)
			inputFile = &file
		}
	}

	if inputFile != nil {
		msg, err := ctx.Bot().SendVideo(ctx, &telego.SendVideoParams{
			ChatID:          meesageParam.ChatID,
			ReplyParameters: meesageParam.ReplyParameters,
			ReplyMarkup:     meesageParam.ReplyMarkup,
			Caption:         meesageParam.Text[:min(1024, len(meesageParam.Text))],
			ParseMode:       meesageParam.ParseMode,
			Video:           tu.FileFromURL(inputFile.URL),
		})
		if err != nil {
			utils.Log.Error(err)

			return 0
		}

		return msg.MessageID
	}

	msg, err := ctx.Bot().SendMessage(ctx, meesageParam)
	if err != nil {
		utils.Log.Error(err)

		return 0
	}

	return msg.MessageID
}

// Редактирование текущего сообщения
func EditCurrentMessage(ctx *th.Context, update telego.Update, text string, args ...any) {
	if (update.Message != nil && update.Message.Photo != nil) ||
		(update.CallbackQuery != nil && update.CallbackQuery.Message.Message().Photo != nil) {
		SendMessage(ctx, true, false, update, text, args...)
		DeleteCurrentMessage(ctx, update)

		return
	}

	EditMessage(ctx, update, GetCurrentMessageID(update), text, args...)
}

// Редактирование указанного сообщения
func EditMessage(ctx *th.Context, update telego.Update, messageID int, text string, args ...any) error {
	if messageID == 0 {
		return nil
	}

	var inputFile *InputVideo

	meesageParam := &telego.EditMessageTextParams{
		ChatID:    tu.ID(GetUserID(update)),
		MessageID: messageID,
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
	}

	for _, v := range args {
		// nolint
		switch v.(type) {
		case *telego.InlineKeyboardMarkup:
			meesageParam.ReplyMarkup = v.(*telego.InlineKeyboardMarkup)
		case InputVideo:
			file := v.(InputVideo)
			inputFile = &file
		}
	}

	if inputFile == nil {
		_, err := ctx.Bot().EditMessageText(ctx, meesageParam)
		return err
	}

	_, err := ctx.Bot().EditMessageMedia(ctx, &telego.EditMessageMediaParams{
		ChatID:      meesageParam.ChatID,
		MessageID:   meesageParam.MessageID,
		ReplyMarkup: meesageParam.ReplyMarkup,
		Media: &telego.InputMediaVideo{
			Type:      telego.MediaTypeVideo,
			Caption:   truncateText(meesageParam.Text, 1024),
			ParseMode: meesageParam.ParseMode,
			Media: telego.InputFile{
				URL: inputFile.URL,
			},
		},
	})

	return err
}

// Удаление текущего сообщения
func DeleteCurrentMessage(ctx *th.Context, update telego.Update) {
	DeleteMessage(ctx, update, GetCurrentMessageID(update))
}

// Удаление сообщения
func DeleteMessage(ctx *th.Context, update telego.Update, messageID int) {
	if err := ctx.Bot().DeleteMessage(ctx, &telego.DeleteMessageParams{
		ChatID:    tu.ID(GetUserID(update)),
		MessageID: messageID,
	}); err != nil {
		utils.Log.Error(err)
	}
}

// Отправка сообщения ответа на callback
func AnswerCallbackQuery(ctx *th.Context, update telego.Update, text string) {
	if update.CallbackQuery == nil {
		return
	}

	if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(update.CallbackQuery.ID).WithText(text)); err != nil {
		utils.Log.Error(err)
	}
}

func truncateText(s string, limit int) string {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit])
	}

	return s
}
