package types

import (
	"bot-api/util"
	"errors"
	"path/filepath"
	"strings"
)

const botDirectory = "bot"

type BotConfig struct {
	File    string `yaml:"file"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile"`
}

type UserConfig struct {
	File    string `yaml:"file"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile"`
}

type Message struct {
	Reply  string `json:"reply,omitempty"`
	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`
}

func NewBot(bot string) (*BotConfig, error) {
	if bot == "" {
		return nil, errors.New("empty bot")
	}
	configPath := filepath.Join(botDirectory, bot, "0.yaml")
	config, err := util.LoadYaml[BotConfig](configPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.Name) == "" {
		return nil, errors.New("bot configuration requires a non-empty name")
	}
	if strings.TrimSpace(config.Profile) == "" {
		return nil, errors.New("bot configuration requires a non-empty profile")
	}
	config.File = configPath
	return config, nil
}

func NewUser(bot *BotConfig, user string) (*UserConfig, error) {
	if user == "" {
		return nil, errors.New("empty user")
	}
	configPath := filepath.Join(filepath.Dir(bot.File), user+".yaml")
	config, err := util.LoadYaml[UserConfig](configPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(config.Name) == "" {
		return nil, errors.New("user configuration requires a non-empty name")
	}
	if strings.TrimSpace(config.Profile) == "" {
		return nil, errors.New("user configuration requires a non-empty profile")
	}
	config.File = configPath
	return config, nil
}
