package config

type Config struct {
	Application Application `yaml:"Application" env:"APP" flag:""`
}

type Application struct {
	LogLevel   string `yaml:"LogLevel" env:"LOGLEVEL"`
	TGBotToken string `yaml:"TGBotToken" env:"TG_BOT_TOKEN" flag:"tg-bot-token" usage:"Токен телегам бота"`
	Port       int    `yaml:"Port" env:"PORT" flag:"port" usage:"Порт запуска api прокси"`
	Domen      string `yaml:"Domen" env:"DOMEN" flag:"domen" usage:"Домен к которому будет обращаться Телеглрам для прокси запроса Ютуб видео"`
	ProxyURL   string `yaml:"ProxyURL" env:"PROXY_URL" flag:"proxy-url" cli:"optional" usage:"Прокси для отправки запросов"`
}
