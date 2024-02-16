package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type OmnivoreService struct {
	Name   string
	Config OmnivoreConfig
}

type OmnivoreConfig struct {
	ReloadUUID       string `yaml:"reloadUUID"`
	TargetFolderUUID string `yaml:"targetFolderUUID"`
	Username         string `yaml:"username"`
	ApiKey           string `yaml:"apiKey"`
	Query            string `yaml:"query"`
}

type searchPayload struct {
	Query     string                 `json:"query"`
	Variables searchPayloadVariables `json:"variables"`
}

type searchPayloadVariables struct {
	After string `json:"after"`
	First int    `json:"first"`
	Query string `json:"query"`
}

type omnivoreItem struct {
	Title       string              `json:"title"`
	Author      string              `json:"author"`
	Slug        string              `json:"slug"`
	PageType    string              `json:"pageType"`
	PublishedAt time.Time           `json:"publishedAt"`
	SavedAt     time.Time           `json:"savedAt"`
	URL         *url.URL            `json:"url"`
	Labels      []searchResultLabel `json:"labels"`
}

type searchResultData struct {
	Data searchResultSearch
}

type searchResultSearch struct {
	Search searchResultEdges
}

type searchResultEdges struct {
	Edges []searchResultNodeList
}

type searchResultNodeList struct {
	Node searchResultNode
}

type searchResultNode struct {
	Title       string              `json:"title"`
	Author      string              `json:"author"`
	Slug        string              `json:"slug"`
	PageType    string              `json:"pageType"`
	PublishedAt string              `json:"publishedAt"`
	SavedAt     string              `json:"savedAt"`
	URL         string              `json:"url"`
	Labels      []searchResultLabel `json:"labels"`
}

type searchResultLabel struct {
	Name string `json:"name"`
}

type articlePayload struct {
	Query     string                  `json:"query"`
	Variables articlePayloadVariables `json:"variables"`
}

type articlePayloadVariables struct {
	Username string `json:"username"`
	Slug     string `json:"slug"`
}

type omnivoreArticle struct {
	Id      string `json:"id"`
	Url     string `json:"url"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Content string `json:"content"`
}

type articleResultData struct {
	Data articleResultOuterArticle `json:"data"`
}

type articleResultOuterArticle struct {
	Article articleResultArticle `json:"article"`
}

type articleResultArticle struct {
	Article omnivoreArticle `json:"article"`
}

func (s OmnivoreService) GenerateFiles(maxArticles uint) error {
	fmt.Println("inside generateFiles (omnivore)")
	rm := Remarkable{Config: s.GetRemarkableConfig()}
	searchResults, err := s.getSearchResults()
	if err != nil {
		fmt.Println("Could not get omnivore articles: ", err)
		return err
	}

	var processed uint = 0
	for _, searchResult := range searchResults {
		fileName := getFilename(searchResult.SavedAt, searchResult.Title)
		extension := filepath.Ext(searchResult.URL.String())
		fmt.Println(fileName, extension)
		if extension == ".pdf" {
			fileContent := createPDFFileContent(searchResult.URL.String())
			rm.generatePDF(fileName, fileContent)
		} else {
			article, err := s.getArticleContent(searchResult.Slug)
			if err != nil {
				fmt.Println(fmt.Sprintf("Could not get readable article: %s (%s)", err, searchResult.URL))
				//s.registerHandled(pocketItem)
				continue
			}
			fileContent := createEpubFileContent(article.Title, article.Content, article.Author)
			rm.generateEpub(fileName, fileContent)
		}

		//s.registerHandled(pocketItem)
		processed++
		fmt.Println(fmt.Sprintf("progress: %d/%d", processed, maxArticles))
		if processed == maxArticles {
			break
		}
	}

	return nil
}

func (s OmnivoreService) GetRemarkableConfig() *RemarkableConfig {
	return &RemarkableConfig{
		Service:          s.Name,
		ReloadUUID:       s.Config.ReloadUUID,
		TargetFolderUUID: s.Config.TargetFolderUUID,
	}
}

func (s OmnivoreService) getSearchResults() ([]omnivoreItem, error) {
	config := s.Config

	retrieveResult := &searchResultData{}

	query := "query Search($after: String, $first: Int, $query: String) { search(first: $first, after: $after, query: $query) { ... on SearchSuccess { edges { node { title author slug pageType publishedAt savedAt url labels {name} } } } ... on SearchError { errorCodes } } }"
	variables := searchPayloadVariables{
		"0",
		10,
		config.Query,
	}

	body, _ := json.Marshal(searchPayload{query, variables})

	req, _ := http.NewRequest("POST", "https://api-prod.omnivore.app/api/graphql", bytes.NewReader(body))
	req.Header.Add("X-Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", config.ApiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []omnivoreItem{}, err
	}

	if resp.StatusCode != 200 {
		return []omnivoreItem{}, fmt.Errorf("got response %d; X-Error=[%s]", resp.StatusCode, resp.Header.Get("X-Error"))
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(retrieveResult)
	if err != nil {
		return []omnivoreItem{}, err
	}

	var items []omnivoreItem
	for _, item := range retrieveResult.Data.Search.Edges {
		parsedURL, _ := url.Parse(item.Node.URL)
		parsedPublishedAt, _ := time.Parse(time.RFC3339, item.Node.PublishedAt)
		parsedSavedAt, _ := time.Parse(time.RFC3339, item.Node.SavedAt)
		items = append(items, omnivoreItem{
			item.Node.Title,
			item.Node.Author,
			item.Node.Slug,
			item.Node.PageType,
			parsedPublishedAt,
			parsedSavedAt,
			parsedURL,
			item.Node.Labels,
		})
	}

	return items, nil
}

// TODO: Break out Omnivore API call to its own method. Same for title/link prepend code
func (s OmnivoreService) getArticleContent(articleId string) (omnivoreArticle, error) {
	config := s.Config

	retrieveResult := &articleResultData{}

	query := "query GetArticle($username: String! $slug: String!) { article(username: $username, slug: $slug) { ... on ArticleSuccess { article { id url title author content } } } }"
	variables := articlePayloadVariables{
		config.Username,
		articleId,
	}

	body, _ := json.Marshal(articlePayload{query, variables})

	req, _ := http.NewRequest("POST", "https://api-prod.omnivore.app/api/graphql", bytes.NewReader(body))
	req.Header.Add("X-Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", config.ApiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return omnivoreArticle{}, err
	}

	if resp.StatusCode != 200 {
		return omnivoreArticle{}, fmt.Errorf("got response %d; X-Error=[%s]", resp.StatusCode, resp.Header.Get("X-Error"))
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(retrieveResult)
	if err != nil {
		return omnivoreArticle{}, err
	}

	parsedContent, _ := html.Parse(strings.NewReader(retrieveResult.Data.Article.Article.Content))
	bodyTag := dom.QuerySelector(parsedContent, "body")

	articleLink := dom.CreateElement("a")
	dom.SetTextContent(articleLink, retrieveResult.Data.Article.Article.Url)
	dom.SetAttribute(articleLink, "href", retrieveResult.Data.Article.Article.Url)
	dom.PrependChild(bodyTag, articleLink)

	articleTitle := dom.CreateElement("h1")
	dom.SetTextContent(articleTitle, retrieveResult.Data.Article.Article.Title)
	dom.PrependChild(bodyTag, articleTitle)

	retrieveResult.Data.Article.Article.Content = dom.OuterHTML(parsedContent)

	return retrieveResult.Data.Article.Article, nil
}
