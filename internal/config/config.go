package config

import (
	"bot-api/common"
	"errors"
	"path/filepath"
	"strings"
)

const BotDirectory = "bot"

type BotConfig struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile"`
}

type UserConfig struct {
	File    string `yaml:"file"`
	Path    string `yaml:"path"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile"`
}

type Message struct {
	Reply  string `json:"reply,omitempty"`
	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`
}

func NewBotConfig(bot string) (*BotConfig, error) {
	if bot == "" {
		return nil, errors.New("empty bot")
	}
	configPath := filepath.Join(BotDirectory, bot, "_.md")
	config, md, err := common.LoadMarkYaml[BotConfig](configPath)
	if err != nil {
		return nil, err
	}
	config.Path = configPath
	config.Profile = strings.TrimSpace(md)
	config.Name = strings.TrimSpace(config.Name)
	if config.Name == "" {
		return nil, errors.New("bot configuration requires a non-empty name")
	}
	if config.Profile == "" {
		return nil, errors.New("bot configuration requires a non-empty profile")
	}
	return config, nil
}

func NewUserConfig(bot *BotConfig, user string) (*UserConfig, error) {
	if user == "" {
		return nil, errors.New("empty user")
	}
	configPath := filepath.Join(filepath.Dir(bot.Path), user+".md")
	config, md, err := common.LoadMarkYaml[UserConfig](configPath)
	if err != nil {
		return nil, err
	}
	config.Path = configPath
	config.Profile = strings.TrimSpace(md)
	config.Name = strings.TrimSpace(config.Name)
	if config.Name == "" {
		return nil, errors.New("user configuration requires a non-empty name")
	}
	if config.Profile == "" {
		return nil, errors.New("user configuration requires a non-empty profile")
	}
	return config, nil
}

func NewOptionalUser(bot *BotConfig, user string) *UserConfig {
	if user == "" {
		return nil
	}
	userConfig, err := NewUserConfig(bot, user)
	if err != nil {
		return &UserConfig{Name: user}
	}
	return userConfig
}
