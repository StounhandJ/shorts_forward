package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

var (
	configPath      = "config/config.yaml"
	devConfigPath   = "config/config.dev.yaml"
	localConfigPath = "config/config.local.yaml"
)

// nolint
type duration time.Duration

func LoadConfig(c any) error {
	var path string

	switch os.Getenv("ENV") {
	case "local":
		path = localConfigPath
	case "dev":
		path = devConfigPath
	case "prod":
		path = configPath
	default:
		path = configPath
	}

	return parseConfig(c, path, CommonParseOptions)
}

func parseConfig(c any, path string, opts parseOptions) error {
	if err := readFile(c, path); err != nil {
		return err
	}

	return CommonHelp("invest", "Запустить сервер", "", c, opts)
}

func readFile(cfg interface{}, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}

	defer func() {
		if cerr := f.Close(); cerr != nil {
			log.Fatal(cerr)
		}
	}()

	decoder := yaml.NewDecoder(f)

	if err = decoder.Decode(cfg); err != nil {
		return fmt.Errorf("failed to decode yaml file %s: %w", path, err)
	}

	return nil
}

// UnmarshalYAML реализует InterfaceUnmarshaler (UnmarshalYAML(func(interface{}) error) error).
// Поддерживает:
// - строку parseable через time.ParseDuration, например "5m", "1h30m"
// - целое число (интерпретируем как секунды)
// - числовой тип (float) - тоже как секунды с дробной частью
// nolint
func (d *duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// 1) Попробовать как строку ("5m")
	var s string
	if err := unmarshal(&s); err == nil {
		dur, err := time.ParseDuration(s)
		if err != nil {
			return err
		}

		*d = duration(dur)

		return nil
	}

	// 2) Попробовать как int (интерпретируем как секунды)
	var i int64
	if err := unmarshal(&i); err == nil {
		*d = duration(time.Duration(i) * time.Second)

		return nil
	}

	// 3) Попробовать как float64 (секунды с дробной частью)
	var f float64
	if err := unmarshal(&f); err == nil {
		*d = duration(time.Duration(f * float64(time.Second)))

		return nil
	}

	// 4) Попробовать прямой decode в time.Duration (если библиотека отдала nanoseconds)
	var td time.Duration
	if err := unmarshal(&td); err == nil {
		*d = duration(td)

		return nil
	}

	return fmt.Errorf("unsupported duration format")
}

// MarshalYAML - полезно при сериализации обратно в YAML (запишет строку "5m0s").
// nolint
func (d duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}
