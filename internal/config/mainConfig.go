package config

type Config struct {
	Application Application `yaml:"Application" env:"APP" flag:""`
}

type Application struct {
	LogLevel   string `yaml:"LogLevel" env:"LOGLEVEL"`
	TGBotToken string `yaml:"TGBotToken" env:"TG_BOT_TOKEN" flag:"tg-bot-token" usage:"Токен телегам бота"`
	ProxyURL   string `yaml:"ProxyURL" env:"PROXY_URL" flag:"proxy-url" usage:"Прокси для отправки запросов"`
}
