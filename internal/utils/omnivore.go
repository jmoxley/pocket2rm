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
	HandledLabel     string `json:"handledLabel"`
	SkippedLabel     string `json:"skippedLabel"`
}

type searchPayloadVariables struct {
	After string `json:"after"`
	First int    `json:"first"`
	Query string `json:"query"`
}

type omnivoreItem struct {
	Id          string          `json:"id"`
	Title       string          `json:"title"`
	Author      string          `json:"author"`
	Slug        string          `json:"slug"`
	PageType    string          `json:"pageType"`
	PublishedAt time.Time       `json:"publishedAt"`
	SavedAt     time.Time       `json:"savedAt"`
	URL         *url.URL        `json:"url"`
	Labels      []omnivoreLabel `json:"labels"`
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
	Id          string          `json:"id"`
	Title       string          `json:"title"`
	Author      string          `json:"author"`
	Slug        string          `json:"slug"`
	PageType    string          `json:"pageType"`
	PublishedAt string          `json:"publishedAt"`
	SavedAt     string          `json:"savedAt"`
	URL         string          `json:"url"`
	Labels      []omnivoreLabel `json:"labels"`
}

type omnivoreLabel struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type LabelResultData struct {
	Data LabelResultOuterLabels
}

type LabelResultOuterLabels struct {
	Labels LabelResultLabelList
}

type LabelResultLabelList struct {
	Labels []omnivoreLabel `json:"labels"`
}

type omnivorePayload struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables"`
}

type articlePayloadVariables struct {
	Username string `json:"username"`
	Slug     string `json:"slug"`
}

type omnivoreArticle struct {
	Id      string          `json:"id"`
	Url     string          `json:"url"`
	Title   string          `json:"title"`
	Author  string          `json:"author"`
	Content string          `json:"content"`
	Labels  []omnivoreLabel `json:"labels"`
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

type setLabelsResultData struct {
	Data setLabelsResultSetLabels `json:"data"`
}

type setLabelsResultSetLabels struct {
	SetLabels setLabelsResultLabelList `json:"setLabels"`
}

type setLabelsResultLabelList struct {
	Labels []omnivoreLabel `json:"labels"`
}

type setLabelsVariables struct {
	Input setLabelsVariablesInput `json:"input"`
}

type setLabelsVariablesInput struct {
	PageId   string   `json:"pageId"`
	LabelIds []string `json:"labelIds"`
}

func (s OmnivoreService) GetRemarkableConfig() *RemarkableConfig {
	return &RemarkableConfig{
		Service:          s.Name,
		ReloadUUID:       s.Config.ReloadUUID,
		TargetFolderUUID: s.Config.TargetFolderUUID,
	}
}

func (s OmnivoreService) GenerateFiles(maxArticles uint) error {
	config := s.Config

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
				s.registerHandled(searchResult, config.SkippedLabel)
				continue
			}
			fileContent := createEpubFileContent(article.Title, article.Content, article.Author)
			rm.generateEpub(fileName, fileContent)
		}

		s.registerHandled(searchResult, config.HandledLabel)
		processed++
		fmt.Println(fmt.Sprintf("progress: %d/%d", processed, maxArticles))
		if processed == maxArticles {
			break
		}
	}

	return nil
}

func (s OmnivoreService) registerHandled(article omnivoreItem, label string) bool {
	fmt.Println("Marking article as handled")

	// TODO: Only get label list once per session
	labelList, err := s.getLabelList()
	if err != nil {
		fmt.Println("Could not get list of labels, skipping...")
		return false
	}
	labelId := labelList[label]
	articleLabels := article.Labels
	var updatedLabelList []string
	for _, articleLabel := range articleLabels {
		updatedLabelList = append(updatedLabelList, articleLabel.Id)
	}
	updatedLabelList = append(updatedLabelList, labelId)

	retrieveResult := &setLabelsResultData{}

	query := "mutation SetLabels($input: SetLabelsInput!) { setLabels(input: $input) { ... on SetLabelsSuccess { labels { id name } } ... on SetLabelsError { errorCodes } } }"
	variables := setLabelsVariables{
		setLabelsVariablesInput{
			article.Id,
			updatedLabelList,
		},
	}

	resp, err := s.omnivoreRequest(query, variables)
	if err != nil {
		fmt.Println("Could not update article labels")
		return false
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(retrieveResult)
	if err != nil {
		return false
	}

	if len(retrieveResult.Data.SetLabels.Labels) == len(updatedLabelList) {
		fmt.Println(fmt.Sprintf("Added label '%s' to article", label))
		return true
	}

	return false
}

func (s OmnivoreService) getSearchResults() ([]omnivoreItem, error) {
	config := s.Config

	retrieveResult := &searchResultData{}

	query := "query Search($after: String, $first: Int, $query: String) { search(first: $first, after: $after, query: $query) { ... on SearchSuccess { edges { node { id title author slug pageType publishedAt savedAt url labels { id name } } } } ... on SearchError { errorCodes } } }"
	variables := searchPayloadVariables{
		"0",
		10,
		config.Query,
	}

	resp, err := s.omnivoreRequest(query, variables)
	if err != nil {
		return []omnivoreItem{}, err
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
			item.Node.Id,
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

func (s OmnivoreService) getArticleContent(articleId string) (omnivoreArticle, error) {
	config := s.Config

	retrieveResult := &articleResultData{}

	query := "query GetArticle($username: String! $slug: String!) { article(username: $username, slug: $slug) { ... on ArticleSuccess { article { id url title author content labels { id name } } } } }"
	variables := articlePayloadVariables{
		config.Username,
		articleId,
	}

	resp, err := s.omnivoreRequest(query, variables)
	if err != nil {
		return omnivoreArticle{}, err
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

func (s OmnivoreService) getLabelList() (map[string]string, error) {
	fmt.Println("getting label list")
	retrieveResult := &LabelResultData{}

	query := "query GetLabels { labels { ... on LabelsSuccess { labels { ...LabelFields } } ... on LabelsError { errorCodes } } } fragment LabelFields on Label { id name }"
	var variables interface{}

	resp, err := s.omnivoreRequest(query, variables)
	if err != nil {
		return map[string]string{}, err
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(retrieveResult)
	if err != nil {
		return map[string]string{}, err
	}

	labels := map[string]string{}
	for _, label := range retrieveResult.Data.Labels.Labels {
		labels[label.Name] = label.Id
	}

	return labels, nil
}

func (s OmnivoreService) omnivoreRequest(query string, variables interface{}) (*http.Response, error) {
	config := s.Config

	body, _ := json.Marshal(omnivorePayload{query, variables})

	req, _ := http.NewRequest("POST", "https://api-prod.omnivore.app/api/graphql", bytes.NewReader(body))
	req.Header.Add("X-Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", config.ApiKey)

	resp, err := http.DefaultClient.Do(req)

	if resp.StatusCode != 200 {
		err = fmt.Errorf("got response %d; X-Error=[%s]", resp.StatusCode, resp.Header.Get("X-Error"))
	}

	return resp, err
}
