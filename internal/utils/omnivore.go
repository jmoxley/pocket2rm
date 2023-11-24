package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

type OmnivoreService Service

type OmnivoreConfig struct {
	Username string `yaml:"username"`
	ApiKey   string `yaml:"apiKey"`
	Query    string `yaml:"query"`
}

type retrievePayload struct {
	Query     string                   `json:"query"`
	Variables retrievePayloadVariables `json:"variables"`
}

type retrievePayloadVariables struct {
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

func (s OmnivoreService) GenerateFiles(maxArticles uint) error {
	fmt.Println("inside generateFiles (omnivore)")
	searchResults, err := getSearchResults()
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
			generatePDF(fileName, fileContent)
		} else {
			title, XMLcontent, err := getReadableArticle(searchResult.URL)
			if err != nil {
				fmt.Println(fmt.Sprintf("Could not get readable article: %s (%s)", err, searchResult.URL))
				//registerHandled(pocketItem)
				continue
			}
			fileContent := createEpubFileContent(title, XMLcontent)
			generateEpub(fileName, fileContent)
		}

		//registerHandled(pocketItem)
		processed++
		fmt.Println(fmt.Sprintf("progress: %d/%d", processed, maxArticles))
		if processed == maxArticles {
			break
		}
	}

	return nil
}

func getSearchResults() ([]omnivoreItem, error) {
	config := getConfig()

	retrieveResult := &searchResultData{}

	query := "query Search($after: String, $first: Int, $query: String) { search(first: $first, after: $after, query: $query) { ... on SearchSuccess { edges { node { title author slug pageType publishedAt savedAt url labels {name} } } } ... on SearchError { errorCodes } } }"
	variables := retrievePayloadVariables{
		"0",
		10,
		"in:inbox sort:saved-des -label:remarkable -label:RSS",
	}

	body, _ := json.Marshal(retrievePayload{query, variables})

	req, _ := http.NewRequest("POST", "https://api-prod.omnivore.app/api/graphql", bytes.NewReader(body))
	req.Header.Add("X-Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", config.Omnivore.ApiKey)

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