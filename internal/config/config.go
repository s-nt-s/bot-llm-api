package config

import (
	"bot-api/common"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const BotDirectory = "bot"

type BotConfig struct {
	Path    string          `yaml:"path"`
	Name    string          `yaml:"name"`
	Profile string          `yaml:"profile"`
	Schema  json.RawMessage `yaml:"-"`
}

type UserConfig struct {
	File    string `yaml:"file"`
	Path    string `yaml:"path"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile"`
}

type Message struct {
	Reply  string          `json:"reply,omitempty"`
	Json   json.RawMessage `json:"json,omitempty"`
	Status int             `json:"status"`
	Error  string          `json:"error,omitempty"`
	Model  string          `json:"model,omitempty"`
}

func NewBotConfig(bot string) (*BotConfig, error) {
	if bot == "" {
		return nil, errors.New("empty bot")
	}
	botSlug := strings.ToLower(bot)
	configPath := filepath.Join(BotDirectory, botSlug, "_.md")
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

	schemaPath := filepath.Join(filepath.Dir(configPath), "schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read schema %s: %w", schemaPath, err)
	}
	if err == nil {
		if err := json.Unmarshal(schemaData, &config.Schema); err != nil {
			return nil, fmt.Errorf("decode schema %s: %w", schemaPath, err)
		}
		log.Printf("Loaded %s", schemaPath)
	}

	log.Printf("Loaded %s", configPath)
	return config, nil
}

func NewUserConfig(bot *BotConfig, user string) (*UserConfig, error) {
	if user == "" {
		return nil, errors.New("empty user")
	}
	userSlug := strings.ToLower(user)
	configPath := filepath.Join(filepath.Dir(bot.Path), userSlug+".md")

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
	log.Printf("Loaded %s", configPath)
	return config, nil
}

func NewOptionalUser(bot *BotConfig, user string) *UserConfig {
	if user == "" {
		return nil
	}
	userConfig, err := NewUserConfig(bot, user)
	if err != nil {
		log.Printf("Error loading user %s %s", user, err.Error())
		return &UserConfig{Name: user}
	}
	return userConfig
}
