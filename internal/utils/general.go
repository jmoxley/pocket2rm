package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bmaupin/go-epub"
	"github.com/go-shiori/dom"
	"github.com/go-shiori/go-readability"
	"github.com/google/uuid"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Service  string         `yaml:"service"`
	Pocket   PocketConfig   `yaml:"pocket,omitempty"`
	Omnivore OmnivoreConfig `yaml:"omnivore,omitempty"`
}

type ReaderService interface {
	GenerateFiles(maxArticles uint) error
	GetRemarkableConfig() *RemarkableConfig
}

type Time time.Time

func (t *Time) UnmarshalJSON(b []byte) error {
	i, err := strconv.ParseInt(string(bytes.Trim(b, `"`)), 10, 64)
	if err != nil {
		return err
	}

	*t = Time(time.Unix(i, 0))

	return nil
}

func cleanDuplicateAtrtibutess(doc *html.Node, attrName string) string {
	var cleanId func(*html.Node, int)

	cleanId = func(node *html.Node, idx int) {
		attribute := dom.GetAttribute(node, attrName)
		dom.RemoveAttribute(node, attrName)
		dom.SetAttribute(node, attrName, attribute)
	}

	nodeList := dom.QuerySelectorAll(doc, "["+attrName+"]")
	dom.ForEachNode(nodeList, cleanId)

	return dom.OuterHTML(doc)
}

func createEpubFileContent(title string, content string) []byte {
	e := epub.NewEpub(title)
	e.SetAuthor("pocket2rm")
	_, _ = e.AddSection(content, title, "", "")

	tmpName := "/tmp/epub" + uuid.New().String()[0:5] + ".epub"
	_ = e.Write(tmpName)
	defer os.Remove(tmpName)

	fileContent, _ := os.ReadFile(tmpName)
	return fileContent
}

func createPDFFileContent(url string) []byte {
	resp, _ := http.Get(url)
	defer resp.Body.Close()
	content, _ := io.ReadAll(resp.Body)
	return content
}

func GetConfig() *Config {
	fileContent, _ := os.ReadFile(getConfigPath())
	var config *Config
	yaml.Unmarshal(fileContent, &config)

	return config
}

func getConfigPath() string {
	userHomeDir := getUserHomeDir()

	return filepath.Join(userHomeDir, ".pocket2rm")
}

// generate filename from time added and title
func getFilename(timeAdded time.Time, title string) string {
	// fileType: "epub" or "pdf"
	title = strings.Join(strings.Fields(title), "-")
	title = strings.Replace(title, "/", "_", -1)
	fileName := fmt.Sprintf("%s_%s", timeAdded.Format("20060102"), title)
	return fileName
}

func getReadableArticle(url *url.URL) (string, string, error) {
	timeout, _ := time.ParseDuration("30s")
	article, err := readability.FromURL(url.String(), timeout)

	if err != nil {
		return "", "", err
	}

	// Strip duplicate attributes from tags
	if article.Node != nil {
		article.Content = cleanDuplicateAtrtibutess(article.Node, "id")
		article.Content = cleanDuplicateAtrtibutess(article.Node, "alt")
	}

	// Include title and source URL in beginning of content
	content := fmt.Sprintf(`<h1> %s </h1>
		<a href="%s">%s</a>
		%s`, article.Title, url.String(), url.String(), article.Content)

	return article.Title, content, nil
}

func GetService(cfg *Config) (ReaderService, error) {
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
	appConfig := GetConfig()

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
