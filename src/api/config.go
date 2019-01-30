package api

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-errors/errors"
	configlib "github.com/gookit/config"
	"github.com/gookit/config/yaml"
)

type GoalConfig = configlib.Config

var config *GoalConfig

var configPath = os.Getenv("GOAL_CONFIGS")
var goalEnv = os.Getenv("GOAL_ENV")

func NewEmptyConfig(prefix string) *GoalConfig {
	return configlib.NewEmpty(prefix)
}

// LoadConfig primes the server's configuration file(s)
func LoadConfig() *GoalConfig {
	if config != nil {
		return config
	}
	config = configlib.NewEmpty("goal")

	if configPath == "" {
		configPath = "./configs"
	}

	config.WithOptions(configlib.ParseEnv)
	config.AddDriver(yaml.Driver)

	err := loadConfig("%s/default.yml", configPath)
	if err != nil {
		log.Panicf("Failed load default config: %v", err)
	}

	if goalEnv != "" {
		err := loadConfig("%s/%s.yml", configPath, goalEnv)
		if err != nil {
			log.Panicf("Failed to load environment configuration: %v", err)
		}
	}

	err = loadConfig("%s/custom.yml", configPath)
	if err != nil {
		log.Panicf("Failed to load custom configuration: %v", err)
	}

	for _, envEntry := range os.Environ() {
		if !strings.HasPrefix(envEntry, "GOAL_") {
			continue
		}

		keyVal := strings.Split(envEntry, "=")
		key := strings.Replace(
			strings.ToLower(keyVal[0]),
			"_",
			".",
			-1,
		)
		val := strings.Join(keyVal[1:], "=")
		config.Set(key, val)
	}

	return config
}

func GetSubconfig(path string, config *GoalConfig) (*GoalConfig, error) {
	subpath := fmt.Sprintf("goal.%s", path)
	data := config.Get(subpath)
	subconfig := NewEmptyConfig(subpath)

	if data == nil {
		return subconfig, nil
	}

	// Todo: I am not sure why we need both map[string] and map[interface]
	switch data.(type) {
	case map[string]interface{}:
		for key, val := range data.(map[string]interface{}) {
			err := subconfig.Set(key, val)
			if err != nil {
				return nil, errors.Wrap(err, 0)
			}
		}
	default:
		for key, val := range data.(map[interface{}]interface{}) {
			err := subconfig.Set(key.(string), val)
			if err != nil {
				return nil, errors.Wrap(err, 0)
			}
		}
	}

	return subconfig, nil
}

func loadConfig(str string, args ...interface{}) error {
	file := fmt.Sprintf(str, args...)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil
	}

	return config.LoadFiles(file)
}
