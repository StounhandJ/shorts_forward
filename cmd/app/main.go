package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/StounhandJ/shorts_forward/internal/config"
	downloadersService "github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/StounhandJ/shorts_forward/internal/downloaders/instagram"
	tiktok "github.com/StounhandJ/shorts_forward/internal/downloaders/tik_tok"
	"github.com/StounhandJ/shorts_forward/internal/downloaders/youtube"
	"github.com/StounhandJ/shorts_forward/internal/handlers"
	"github.com/StounhandJ/shorts_forward/internal/utils"
	"github.com/StounhandJ/shorts_forward/internal/ytdlp"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"github.com/valyala/fasthttp"
)

var cfg config.Config

func main() {
	//------ Получение Конфигурации ------//
	if err := config.LoadConfig(&cfg); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	utils.InitLogger(cfg.Application.LogLevel)
	//---------------//

	//------ TELEGRAM бот ------//
	utils.Log.Info("Подключение TG-бота")

	bot, err := telego.NewBot(cfg.Application.TGBotToken, telego.WithDefaultLogger(cfg.Application.LogLevel == "debug", true))
	if err != nil {
		utils.Log.Error(err)
		os.Exit(1)
	}

	// Обработка сообщений ботом
	updates, err := bot.UpdatesViaLongPolling(context.Background(), nil)
	if err != nil {
		utils.Log.Error(err)
		os.Exit(1)
	}

	bh, err := th.NewBotHandler(bot, updates)
	if err != nil {
		utils.Log.Error(err)
		os.Exit(1)
	}

	yt := ytdlp.New("yt-dlp")

	youtubeDownloader := youtube.New(cfg.Application.Domain, yt)
	instagramDownloader := instagram.New(cfg.Application.Domain, yt)
	tiktokDownloader := tiktok.New(cfg.Application.Domain, yt)
	handler := handlers.NewHandler([]downloadersService.IDownloader{
		youtubeDownloader,
		instagramDownloader,
		tiktokDownloader,
	})
	httpHandler := handlers.NewHttpHandler([]downloadersService.IDownloader{
		youtubeDownloader,
		instagramDownloader,
		tiktokDownloader,
	})

	handler.SetupRoutes(bh)

	user, err := bot.GetMe(context.Background())
	if err != nil {
		utils.Log.Error(err)
		os.Exit(1)
	}

	go func() {
		fmt.Printf(
			"TG БОТ ID=%d имя=%s username=@%s\n",
			user.ID,
			user.FirstName,
			user.Username,
		)
		utils.Log.Fatal(bh.Start())
	}()

	go func() {
		fmt.Printf(
			"API Прослушивание порта %d\n", cfg.Application.Port,
		)

		server := fasthttp.Server{
			LogAllErrors: false,
			Handler:      httpHandler.Handler,
		}

		utils.Log.Fatal(server.ListenAndServe(":" + strconv.Itoa(cfg.Application.Port)))
	}()
	//---------------//

	//------ Ожидание заершения программы ------//
	utils.Log.Info("Всё запущено")

	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	<-cSignal
}
