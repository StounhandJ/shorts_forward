package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/StounhandJ/shorts_forward/internal/config"
	"github.com/stretchr/testify/require"
)

var cfg = configTestStruct{
	API:     configAPI{WithEnv: "1", Usage: "1"},
	WithEnv: "1",
}

type (
	configTestStruct struct {
		API configAPI `yaml:"api" category:"ConfigAPI"`

		WithEnv    string `yaml:"test-with-env" env:"TEST-WITH-ENV"`
		EnvPrefix  string `yaml:"env_prefix" envprefix:"ENVPREFIX" default:"env"`
		NewFlag    uint16 `yaml:"new_flag" flag:"tag-new-flag"`
		FlagPrefix string `yaml:"flag_prefix" flagprefix:"flag-prefix" default:"flag"`
	}

	storage struct {
		User     string `yaml:"user" cli:"required" default:"U"`
		Database int    `yaml:"database" cli:"required" default:"11"`
	}

	configAPI struct {
		WithoutEnv string `yaml:"without_env" env:"-" default:"1"`
		WithEnv    string `yaml:"test-with-env" env:"TEST-WITH-ENV"`

		Usage string `yaml:"usage" usage:"usage-usage"`
		Host  string `yaml:"host"`

		Env uint16 `yaml:"env" env:"NEW_ENV"`

		EnvPrefix string `yaml:"env_prefix" envprefix:"CONFIG_ENVPREFIX" default:"env"`

		NewFlag    uint16 `yaml:"new_flag" flag:"tag-new-flag"`
		FlagPrefix string `yaml:"flag_prefix" flagprefix:"flag-prefix" default:"flag"`

		Ignore         string `yaml:"ignore" cli:"-"`
		Hidden         uint16 `yaml:"hidden" cli:"hidden,optional"`
		UintOptional   uint16 `yaml:"uint_optional" cli:"optional"`
		StringOptional string `yaml:"string_optional" cli:"optional"`
	}
)

func TestHelpDefaultParseOptions(t *testing.T) {
	os.Args = []string{os.Args[0], "-help"}

	helpWasCalled, err := config.WorkHelp("a", "b", "c", &cfg, config.DefaultParseOptions)
	require.NoError(t, err)
	require.True(t, helpWasCalled, "help was not called")
}

func TestHelpCommonParseOptions(t *testing.T) {
	os.Args = []string{os.Args[0], "-help"}

	helpWasCalled, err := config.WorkHelp("a", "b", "c", &cfg, config.CommonParseOptions)
	require.NoError(t, err)
	require.True(t, helpWasCalled, "help was not called")
}

func TestCommonParseHelpNotCalled(t *testing.T) {
	os.Args = []string{os.Args[0]}

	helpWasCalled, err := config.WorkHelp("a", "b", "c", &struct{}{}, config.CommonParseOptions)
	require.NoError(t, err)
	require.False(t, helpWasCalled, "help should not called")
}

func TestCommonParseOptionsRequiredDefaultValuesFromConfig(t *testing.T) {
	os.Args = []string{os.Args[0], "command"}

	cfg := storage{}
	_, err := config.WorkHelp("a", "b", "c", &cfg, config.CommonParseOptions)

	require.Error(t, err)
}

func TestDefaultParseOptionsRequiredDefaultValues(t *testing.T) {
	os.Args = []string{os.Args[0], "command"}

	cfg := storage{}
	_, err := config.WorkHelp("a", "b", "c", &cfg, config.DefaultParseOptions)
	require.NoError(t, err)
	require.NotZero(t, cfg.User, "user should not be empty")
	require.NotZero(t, cfg.Database, "database should not be empty")
}

func TestCommonParseOptionsWithEnvAndFlag(t *testing.T) {
	flag := "1"
	storageUser := "USER"
	password := "PASSWORD"

	os.Args = []string{os.Args[0], "command", fmt.Sprintf("-flag=%s", flag)}
	err := os.Setenv("PASSWORD", password)
	require.NoError(t, err)
	defer os.Clearenv()

	type configTest struct {
		Storage  storage `yaml:"Storage"`
		Password string
		Debug    bool
		Flag     string
	}

	cfg := configTest{Storage: storage{User: storageUser, Database: 1}}

	helpWasCalled, err := config.WorkHelp("a", "b", "c", &cfg, config.CommonParseOptions)
	require.NoError(t, err)

	require.False(t, helpWasCalled, "help was not called")
	require.Equal(t, password, cfg.Password, "password from env is not set")
	require.Equal(t, storageUser, cfg.Storage.User, "user from env is not set")
	require.Equal(t, flag, cfg.Flag, "flag from args is not set")
}

func TestCommonParseOptionsFlagIsDisabled(t *testing.T) {
	os.Args = []string{os.Args[0], "command"}

	type configTest struct {
		Flag string `flag:"-"`
	}

	cfg := configTest{}

	_, err := config.WorkHelp("a", "b", "c", &cfg, config.CommonParseOptions)
	require.Error(t, err)
	require.Zero(t, cfg.Flag, "flag from args is not set")
}
