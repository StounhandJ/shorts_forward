package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

type parseOptions struct {
	Parent                  *parseOptions
	EnvPrefix               string
	EnvIsDisabled           bool
	FlagPrefix              string
	Category                string
	AlreadyHasDefaultValues bool
	RequiredByDefault       bool
}

// Для приложений с yaml конфигом + env
var CommonParseOptions = parseOptions{
	AlreadyHasDefaultValues: true,
	RequiredByDefault:       true,
}

// Для приложений только с env
var DefaultParseOptions = parseOptions{
	RequiredByDefault: true,
}

var (
	tagNameEnv        = "env"        // полностью меняет часть после префикса для env. env:"-" - убрать ввод значения через env.
	tagNameEnvPrefix  = "envprefix"  // полностью перезаписывает префикс env (envprefix:"APP2", envprefix:"")
	tagNameFlag       = "flag"       // полностью меняет часть после префикса для флага. flag:"-" - убрать ввод значения через флаг
	tagNameFlagPrefix = "flagprefix" // полностью перезаписывает префикс флага
	tagNameCLI        = "cli"        // опции через запятую: hidden,required,optional. cli:"-" - игнор поля.
	tagNameUsage      = "usage"      // описание (usage:"делает что-то")
	tagNameDefault    = "default"    // дефолт значение (default:"10")
	tagNameCategory   = "category"   // категория в команде help
)

// Для приложений без субкоманд.
// Если нужны субкоманды, используй urfave/cli напрямую или создай где-нибудь здесь абстракцию.
// opts:
//   - CommonParseOptions - Для приложений с yaml конфигом + env.
//   - DefaultParseOptions - Для приложений только с env.
//   - или сам собери структуру.
//
// Пример:
//
//	CommonHelp("common", "common utils", "common utils for other apps", &cfg, CommonParseOptions)
func CommonHelp(name, usage, description string, cfg any, opts parseOptions) error {
	helpWasCalled, err := WorkHelp(name, usage, description, cfg, opts)
	if helpWasCalled && err == nil {
		os.Exit(0)
	}

	return err
}

func WorkHelp(name, usage, description string, cfg any, opts parseOptions) (bool, error) {
	flags, err := parseFlags(cfg, opts)
	if err != nil {
		return false, fmt.Errorf("ParseFlags: %w", err)
	}

	var helpWasCalled bool

	original := cli.HelpPrinterCustom
	cli.HelpPrinterCustom = func(w io.Writer, templ string, data any, customFunc map[string]any) {
		helpWasCalled = true

		original(w, templ, data, customFunc)
	}

	cmd := &cli.Command{
		Name:        name,
		Usage:       usage,
		Description: description,
		Flags:       flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		return helpWasCalled, fmt.Errorf("cmd.Run: %w", err)
	}

	return helpWasCalled, nil
}

func parseFlags(c any, opts parseOptions) ([]cli.Flag, error) {
	if c == nil {
		return nil, errors.New("config must not be nil")
	}

	v := reflect.ValueOf(c)

	if v.Kind() != reflect.Ptr {
		return nil, errors.New("config must be pointer")
	}

	v = v.Elem()

	if v.Kind() != reflect.Struct {
		return nil, errors.New("config must be struct")
	}

	t := reflect.TypeOf(c).Elem()

	flags := make([]cli.Flag, 0, v.NumField())

	for i := range v.NumField() {
		res, err := parseField(t.Field(i), v.Field(i), opts)
		if err != nil {
			return nil, err
		}

		flags = append(flags, res...)
	}

	return flags, nil
}

type flagOptions[T any] struct {
	Value T
	Dest  *T
	flagOptionsCommon
}

type flagOptionsCommon struct {
	Name       string
	Category   string
	HasValue   bool
	Env        string
	DisableEnv bool
	Usage      string
	Required   bool
	Hidden     bool
}

// nolint: gocyclo, cyclop
func parseField(
	t reflect.StructField,
	v reflect.Value,
	opts parseOptions,
) ([]cli.Flag, error) {
	var flagPrefix, envPrefix string

	if v, ok := t.Tag.Lookup(tagNameFlagPrefix); ok {
		opts.FlagPrefix = v
	}

	if v, ok := t.Tag.Lookup(tagNameEnvPrefix); ok {
		opts.EnvPrefix = v
	}

	if opts.FlagPrefix != "" {
		flagPrefix = opts.FlagPrefix + "-"
	}

	if opts.EnvPrefix != "" {
		envPrefix = opts.EnvPrefix + "_"
	}

	argName, ok := t.Tag.Lookup(tagNameFlag)
	switch {
	case !ok:
		argName = flagPrefix + toKebabCase(t.Name)
	case argName == "-":
		argName = ""
	default:
		argName = flagPrefix + argName
	}

	disableEnv := opts.EnvIsDisabled

	var envName string

	if !disableEnv {
		envName, ok = t.Tag.Lookup(tagNameEnv)
		if !ok {
			envName = envPrefix + toScreamingSnakeCase(t.Name)
		} else {
			if envName == "-" {
				disableEnv = true
			} else {
				envName = envPrefix + envName
			}
		}
	}

	category, ok := t.Tag.Lookup(tagNameCategory)
	switch {
	case ok && v.Kind() != reflect.Struct:
		return nil, fmt.Errorf("category tag is allowed only for structures")
	case !ok && v.Kind() == reflect.Struct:
		category = t.Name
	case !ok && v.Kind() != reflect.Struct:
		category = opts.Category
	}

	if !v.CanSet() {
		return nil, fmt.Errorf("private field: %s", t.Name)
	}

	var defaultValue string

	var hasDefaultValue bool
	if !opts.AlreadyHasDefaultValues {
		defaultValue, hasDefaultValue = t.Tag.Lookup(tagNameDefault)
	}

	usage, _ := t.Tag.Lookup(tagNameUsage)

	var (
		cliRequired bool
		cliOptional bool
		cliHidden   bool
	)

	cliOptionsStr, _ := t.Tag.Lookup(tagNameCLI)
	if cliOptionsStr == "-" {
		return nil, nil
	}

	if cliOptionsStr != "" {
		cliOptions := strings.Split(cliOptionsStr, ",")
		cliRequired = slices.Contains(cliOptions, "required")
		cliOptional = slices.Contains(cliOptions, "optional")
		cliHidden = slices.Contains(cliOptions, "hidden")
	}

	if !cliOptional {
		cliRequired = cliRequired || opts.RequiredByDefault
	}

	if cliHidden && cliRequired {
		return nil, fmt.Errorf("flag %v: must not be hidden and required at the same time, add \"optional\" to cli tag", t.Name)
	}

	configValueIsZero := cliRequired && v.IsZero() && v.Kind() != reflect.Bool && opts.AlreadyHasDefaultValues

	foc := flagOptionsCommon{
		Name:       argName,
		Category:   category,
		HasValue:   false,
		Env:        envName,
		DisableEnv: disableEnv,
		Usage:      usage,
		Required:   cliRequired,
		Hidden:     cliHidden,
	}

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	iface := v.Addr().Interface()
	addr := v.Addr()

	// для корректной работы в случаях, когда T1 в конфиге объявлен как "type T1 T2", делается конвертация в T2 для правильной работы каста
	switch v.Kind() {
	case reflect.Slice:
		sv := v.Type().Elem().Kind()
		switch sv {
		case reflect.String:
			var arr []string

			iface := addr.Convert(reflect.TypeOf(&arr)).Interface()

			dst, ok := iface.(*[]string)
			if !ok {
				return nil, fmt.Errorf("failed to cast *[]string: %s", t.Name)
			}

			fo := flagOptions[[]string]{
				flagOptionsCommon: foc,
			}

			if opts.AlreadyHasDefaultValues && !configValueIsZero {
				fo.HasValue = true
				fo.Value = *dst
			} else if hasDefaultValue {
				fo.HasValue = true
				fo.Value = strings.Split(defaultValue, ",")
			}

			if fo.HasValue {
				fo.Required = false
			}

			return []cli.Flag{stringSliceFlag(fo)}, nil
		default:
			return nil, fmt.Errorf("slice type %v is unsupported", sv)
		}

	case reflect.Struct:
		envPrefixFromTag, hasEnvPrefixFromTag := t.Tag.Lookup(tagNameEnv)
		if hasEnvPrefixFromTag {
			envPrefix += envPrefixFromTag
		} else {
			envPrefix += toScreamingSnakeCase(t.Name)
		}

		flagPrefixFromTag, hasFlagPrefixFromTag := t.Tag.Lookup(tagNameFlag)
		if hasFlagPrefixFromTag {
			flagPrefix += flagPrefixFromTag
		} else {
			flagPrefix += toKebabCase(t.Name)
		}

		newOpts := parseOptions{
			Parent:                  &opts,
			Category:                category,
			EnvPrefix:               envPrefix,
			EnvIsDisabled:           opts.EnvIsDisabled || envPrefixFromTag == "-",
			FlagPrefix:              flagPrefix,
			RequiredByDefault:       cliRequired,
			AlreadyHasDefaultValues: opts.AlreadyHasDefaultValues,
		}

		return parseFlags(iface, newOpts)

	case reflect.TypeOf(time.Duration(0)).Kind():
		dur := time.Duration(0)
		iface := addr.Convert(reflect.TypeOf(&dur)).Interface()

		dst, ok := iface.(*time.Duration)
		if !ok {
			return nil, fmt.Errorf("failed to cast *time.Duration: %s", t.Name)
		}

		fo := flagOptions[time.Duration]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := time.ParseDuration(defaultValue)
			if err != nil {
				return nil, fmt.Errorf("invalid duration format for %s: %w", t.Name, err)
			}

			fo.HasValue = true
			fo.Value = v
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{durationFlag(fo)}, nil

	case reflect.String:
		var str string

		iface := addr.Convert(reflect.TypeOf(&str)).Interface()

		dst, ok := iface.(*string)
		if !ok {
			return nil, fmt.Errorf("failed to cast *string: %s", t.Name)
		}

		fo := flagOptions[string]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			fo.HasValue = true
			fo.Value = defaultValue
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{stringFlag(fo)}, nil

	case reflect.Int:
		var tInt int

		iface := addr.Convert(reflect.TypeOf(&tInt)).Interface()

		dst, ok := iface.(*int)
		if !ok {
			return nil, fmt.Errorf("failed to cast *int: %s", t.Name)
		}

		fo := flagOptions[int]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.Atoi(defaultValue)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = v
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{intFlag(fo)}, nil

	case reflect.Int8:
		var tInt8 int8

		iface := addr.Convert(reflect.TypeOf(&tInt8)).Interface()

		dst, ok := iface.(*int8)
		if !ok {
			return nil, fmt.Errorf("failed to cast *int8: %s", t.Name)
		}

		fo := flagOptions[int8]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseInt(defaultValue, 10, 8)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = int8(v)
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{int8Flag(fo)}, nil

	case reflect.Int16:
		var tInt16 int16

		iface := addr.Convert(reflect.TypeOf(&tInt16)).Interface()

		dst, ok := iface.(*int16)
		if !ok {
			return nil, fmt.Errorf("failed to cast *int16: %s", t.Name)
		}

		fo := flagOptions[int16]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseInt(defaultValue, 10, 16)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = int16(v)
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{int16Flag(fo)}, nil

	case reflect.Int32:
		var tInt32 int32

		iface := addr.Convert(reflect.TypeOf(&tInt32)).Interface()

		dst, ok := iface.(*int32)
		if !ok {
			return nil, fmt.Errorf("failed to cast *int32: %s", t.Name)
		}

		fo := flagOptions[int32]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseInt(defaultValue, 10, 32)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = int32(v)
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{int32Flag(fo)}, nil

	case reflect.Int64:
		var tInt64 int64

		iface := addr.Convert(reflect.TypeOf(&tInt64)).Interface()

		dst, ok := iface.(*int64)
		if !ok {
			return nil, fmt.Errorf("failed to cast *int64: %s", t.Name)
		}

		fo := flagOptions[int64]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseInt(defaultValue, 10, 64)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = v
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{int64Flag(fo)}, nil

	case reflect.Uint:
		var tUint uint

		iface := addr.Convert(reflect.TypeOf(&tUint)).Interface()

		dst, ok := iface.(*uint)
		if !ok {
			return nil, fmt.Errorf("failed to cast *uint: %s", t.Name)
		}

		fo := flagOptions[uint]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseUint(defaultValue, 10, 0)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = uint(v)
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{uintFlag(fo)}, nil

	case reflect.Uint8:
		var tUint8 uint8

		iface := addr.Convert(reflect.TypeOf(&tUint8)).Interface()

		dst, ok := iface.(*uint8)
		if !ok {
			return nil, fmt.Errorf("failed to cast *uint8: %s", t.Name)
		}

		fo := flagOptions[uint8]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseUint(defaultValue, 10, 8)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = uint8(v)
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{uint8Flag(fo)}, nil

	case reflect.Uint16:
		var tUint16 uint16

		iface := addr.Convert(reflect.TypeOf(&tUint16)).Interface()

		dst, ok := iface.(*uint16)
		if !ok {
			return nil, fmt.Errorf("failed to cast *uint16: %s", t.Name)
		}

		fo := flagOptions[uint16]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseUint(defaultValue, 10, 16)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = uint16(v)
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{uint16Flag(fo)}, nil

	case reflect.Uint32:
		var tUint32 uint32

		iface := addr.Convert(reflect.TypeOf(&tUint32)).Interface()

		dst, ok := iface.(*uint32)
		if !ok {
			return nil, fmt.Errorf("failed to cast *uint32: %s", t.Name)
		}

		fo := flagOptions[uint32]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseUint(defaultValue, 10, 32)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = uint32(v)
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{uint32Flag(fo)}, nil

	case reflect.Uint64:
		var tUint64 uint64

		iface := addr.Convert(reflect.TypeOf(&tUint64)).Interface()

		dst, ok := iface.(*uint64)
		if !ok {
			return nil, fmt.Errorf("failed to cast *uint64: %s", t.Name)
		}

		fo := flagOptions[uint64]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseUint(defaultValue, 10, 64)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = v
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{uint64Flag(fo)}, nil

	case reflect.Float64:
		var tFloat64 float64

		iface := addr.Convert(reflect.TypeOf(&tFloat64)).Interface()

		dst, ok := iface.(*float64)
		if !ok {
			return nil, fmt.Errorf("failed to cast *float64: %s", t.Name)
		}

		fo := flagOptions[float64]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseFloat(defaultValue, 64)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = v
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{float64Flag(fo)}, nil

	case reflect.Bool:
		var tBool bool

		iface := addr.Convert(reflect.TypeOf(&tBool)).Interface()

		dst, ok := iface.(*bool)
		if !ok {
			return nil, fmt.Errorf("failed to cast *bool: %s", t.Name)
		}

		fo := flagOptions[bool]{
			flagOptionsCommon: foc,
			Dest:              dst,
		}

		if opts.AlreadyHasDefaultValues && !configValueIsZero {
			fo.HasValue = true
			fo.Value = *dst
		} else if hasDefaultValue {
			v, err := strconv.ParseBool(defaultValue)
			if err != nil {
				return nil, err
			}

			fo.HasValue = true
			fo.Value = v
		}

		if fo.HasValue {
			fo.Required = false
		}

		return []cli.Flag{boolFlag(fo)}, nil

	default:
		return nil, fmt.Errorf("type %v is unsupported", v)
	}
}

func stringFlag(opts flagOptions[string]) *cli.StringFlag {
	flag := &cli.StringFlag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func stringSliceFlag(opts flagOptions[[]string]) *cli.StringSliceFlag {
	flag := &cli.StringSliceFlag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func boolFlag(opts flagOptions[bool]) *cli.BoolFlag {
	flag := &cli.BoolFlag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func intFlag(opts flagOptions[int]) *cli.IntFlag {
	flag := &cli.IntFlag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func int8Flag(opts flagOptions[int8]) *cli.Int8Flag {
	flag := &cli.Int8Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func int16Flag(opts flagOptions[int16]) *cli.Int16Flag {
	flag := &cli.Int16Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func int32Flag(opts flagOptions[int32]) *cli.Int32Flag {
	flag := &cli.Int32Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func int64Flag(opts flagOptions[int64]) *cli.Int64Flag {
	flag := &cli.Int64Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func uintFlag(opts flagOptions[uint]) *cli.UintFlag {
	flag := &cli.UintFlag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func uint8Flag(opts flagOptions[uint8]) *cli.Uint8Flag {
	flag := &cli.Uint8Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func uint16Flag(opts flagOptions[uint16]) *cli.Uint16Flag {
	flag := &cli.Uint16Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func uint32Flag(opts flagOptions[uint32]) *cli.Uint32Flag {
	flag := &cli.Uint32Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func uint64Flag(opts flagOptions[uint64]) *cli.Uint64Flag {
	flag := &cli.Uint64Flag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func float64Flag(opts flagOptions[float64]) *cli.FloatFlag {
	flag := &cli.FloatFlag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

func durationFlag(opts flagOptions[time.Duration]) *cli.DurationFlag {
	flag := &cli.DurationFlag{Name: opts.Name, Category: opts.Category, Destination: opts.Dest, Usage: opts.Usage, Required: opts.Required, Hidden: opts.Hidden}
	if opts.HasValue {
		flag.Value = opts.Value
	}

	if !opts.DisableEnv {
		flag.Sources = cli.EnvVars(opts.Env)
	}

	return flag
}

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}

func toKebabCase(str string) string {
	return strings.ReplaceAll(toSnakeCase(str), "_", "-")
}

func toScreamingSnakeCase(str string) string {
	return strings.ToUpper(toSnakeCase(str))
}
