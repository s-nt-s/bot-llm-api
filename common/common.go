package common

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func byteToYaml[T any](data []byte) (T, error) {
	if data == nil {
		var zero T
		return zero, nil
	}
	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		var zero T
		return zero, err
	}
	return config, nil
}

func LoadYaml[T any](configPath string) (*T, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	config, err := byteToYaml[T](data)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func LoadMarkYaml[T any](configPath string) (*T, string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", err
	}

	frontMatter, markdownContent := extractFrontMatter(data)

	config, err := byteToYaml[T](frontMatter)
	if err != nil {
		return nil, "", err
	}

	return &config, string(markdownContent), nil
}

func extractFrontMatter(data []byte) ([]byte, []byte) {
	content := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, content
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			frontMatter := []byte(strings.Join(lines[1:i], "\n"))
			body := []byte(strings.Join(lines[i+1:], "\n"))
			return frontMatter, body
		}
	}

	return nil, content
}

func GetEnvList(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return []string{}
	}

	parts := strings.Fields(value)
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func GetEnv(key string, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

func GetEnvInt(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return defaultValue
	}
	return parsed
}

func GetTime() time.Time {
	tz := GetEnv("TIMEZONE", "UTC")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		panic(fmt.Sprintf("Error loading location %s: %v\n", tz, err))
	}
	now := time.Now().In(loc)
	return now
}

func Rpl(s string, kv map[string]string) string {
	for k, v := range kv {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}
