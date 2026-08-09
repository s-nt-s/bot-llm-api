package util

import (
	"os"

	"gopkg.in/yaml.v3"
)

func LoadYaml[T any](configPath string) (*T, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
