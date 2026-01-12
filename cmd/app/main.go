package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/StounhandJ/shorts_forward/internal/config"
	downloadersService "github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/StounhandJ/shorts_forward/internal/downloaders/instagram"
	tiktok "github.com/StounhandJ/shorts_forward/internal/downloaders/tik_tok"
	"github.com/StounhandJ/shorts_forward/internal/handlers"
	"github.com/StounhandJ/shorts_forward/internal/utils"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
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

	//------ HTTP клиент для отправки запросов ------//
	client := http.Client{}

	if cfg.Application.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.Application.ProxyURL)
		if err != nil {
			utils.Log.Panic(err)
		}

		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL), // прокси
		}
	}
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

	handler := handlers.NewHandler([]downloadersService.IDownloader{
		// youtube.New(&client), // TODO ТГ не может обработать ссылки на CDN ютуба, можно через себя транслировать
		instagram.New(&client),
		tiktok.New(&client),
	})
	handler.SetupRoutes(bh)

	user, err := bot.GetMe(context.Background())
	if err != nil {
		utils.Log.Error(err)
		os.Exit(1)
	}

	go func() {
		fmt.Printf(
			"TG БОТ ID=%d имя=%s username=@%s",
			user.ID,
			user.FirstName,
			user.Username,
		)
		utils.Log.Fatal(bh.Start())
	}()
	//---------------//

	//------ Ожидание заершения программы ------//
	utils.Log.Info("Всё запущено")

	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	<-cSignal
}
