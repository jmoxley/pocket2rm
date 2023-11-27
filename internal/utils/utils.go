package utils

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Service  string         `yaml:"service"`
	Pocket   PocketConfig   `yaml:"pocket,omitempty"`
	Omnivore OmnivoreConfig `yaml:"omnivore,omitempty"`
}

// ReaderService TODO: Possibly split these into separate interfaces to facilitate further reorganization
type ReaderService interface {
	GenerateFiles(maxArticles uint) error
	GetRemarkableConfig() *RemarkableConfig
}

func GetAppConfig() *AppConfig {
	fileContent, _ := os.ReadFile(getConfigPath())
	var config *AppConfig
	_ = yaml.Unmarshal(fileContent, &config)

	return config
}

func getConfigPath() string {
	userHomeDir := getUserHomeDir()

	return filepath.Join(userHomeDir, ".pocket2rm")
}

func GetService(cfg *AppConfig) (ReaderService, error) {
	switch cfg.Service {
	case "omnivore":
		return OmnivoreService{cfg.Service, cfg.Omnivore}, nil
	case "pocket":
		return PocketService{cfg.Service, cfg.Pocket}, nil
	}

	return nil, fmt.Errorf("unknown service: %q", cfg.Service)
}

func getUserHomeDir() string {
	currentUser, err := user.Current()

	if err != nil {
		fmt.Println("Could not get user")
		panic(1)
	}

	return currentUser.HomeDir
}

func writeRemarkableConfig(rmConfig *RemarkableConfig) {
	configPath := getConfigPath()
	appConfig := GetAppConfig()

	switch rmConfig.Service {
	case "omnivore":
		appConfig.Omnivore.ReloadUUID = rmConfig.ReloadUUID
		appConfig.Omnivore.TargetFolderUUID = rmConfig.TargetFolderUUID
	case "pocket":
		appConfig.Pocket.ReloadUUID = rmConfig.ReloadUUID
		appConfig.Pocket.TargetFolderUUID = rmConfig.TargetFolderUUID
	}

	ymlContent, _ := yaml.Marshal(appConfig)
	_ = os.WriteFile(configPath, ymlContent, os.ModePerm)
}

func writeFile(fileName string, fileContent []byte) {

	// write the whole body at once
	err := os.WriteFile(fileName, fileContent, 0644)
	if err != nil {
		panic(err)
	}
}
