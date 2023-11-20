package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"time"
)

type PocketConfig struct {
	ConsumerKey   string            `yaml:"consumerKey"`
	AccessToken   string            `yaml:"accessToken"`
	RequestParams map[string]string `yaml:"requestParams"`
}

type ByAdded []pocketItem

func (a ByAdded) Len() int           { return len(a) }
func (a ByAdded) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByAdded) Less(i, j int) bool { return a[i].added.Before(a[j].added) }

type Item struct {
	ItemID        string               `json:"item_id"`
	ResolvedID    string               `json:"resolved_id"`
	GivenURL      string               `json:"given_url"`
	ResolvedURL   string               `json:"resolved_url"`
	GivenTitle    string               `json:"given_title"`
	ResolvedTitle string               `json:"resolved_title"`
	IsArticle     int                  `json:"is_article,string"`
	TimeAdded     Time                 `json:"time_added"`
	Tags          map[string]PocketTag `json:"tags"`
}

func (item Item) Title() string {
	title := item.ResolvedTitle
	if title == "" {
		title = item.GivenTitle
	}
	return title
}

type pocketItem struct {
	id    string
	url   *url.URL
	added time.Time
	title string
	tags  map[string]PocketTag
}

type PocketModify struct {
	ConsumerKey string                `json:"consumer_key"`
	AccessToken string                `json:"access_token"`
	Actions     []PocketModifyActions `json:"actions"`
}

type PocketModifyActions struct {
	Action string `json:"action"`
	ItemID string `json:"item_id"`
	Tags   string `json:"tags"`
}

type PocketModifyResult struct {
	Results []string
	Errors  []string
	Status  int
}

type PocketResult struct {
	List     map[string]Item
	Status   int
	Complete int
	Since    int
}

type PocketRetrieve struct {
	ConsumerKey string `json:"consumer_key"`
	AccessToken string `json:"access_token"`
	Count       string `json:"count"`
	ContentType string `json:"contentType"`
	DetailType  string `json:"detailType"`
	Sort        string `json:"sort"`
}

type PocketTag struct {
	ItemId string `json:"item_id"`
	Tag    string `json:"tag"`
}

type Transform struct {
	M11 int `json:"m11"`
	M12 int `json:"m12"`
	M13 int `json:"m13"`
	M21 int `json:"m21"`
	M22 int `json:"m22"`
	M23 int `json:"m23"`
	M31 int `json:"m31"`
	M32 int `json:"m32"`
	M33 int `json:"m33"`
}

func getPocketItems() ([]pocketItem, error) {
	// unfortunately cannot use github.com/motemen/go-pocket
	// because of 32bit architecture
	// Item.ItemID in github.com/motemen/go-pocket is int, which cannot store enough
	// therefore the necessary types and functions have been copied and adapted

	config := getConfig()

	retrieveResult := &PocketResult{}

	body, _ := json.Marshal(PocketRetrieve{
		config.Pocket.ConsumerKey,
		config.Pocket.AccessToken,
		config.Pocket.RequestParams["count"],
		config.Pocket.RequestParams["contentType"],
		config.Pocket.RequestParams["detailType"],
		config.Pocket.RequestParams["sort"],
	})

	req, _ := http.NewRequest("POST", "https://getpocket.com/v3/get", bytes.NewReader(body))
	req.Header.Add("X-Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []pocketItem{}, err
	}

	if resp.StatusCode != 200 {
		return []pocketItem{}, fmt.Errorf("got response %d; X-Error=[%s]", resp.StatusCode, resp.Header.Get("X-Error"))
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(retrieveResult)
	if err != nil {
		return []pocketItem{}, err
	}

	var items []pocketItem
	for id, item := range retrieveResult.List {
		parsedURL, _ := url.Parse(item.ResolvedURL)
		items = append(items, pocketItem{id, parsedURL, time.Time(item.TimeAdded), item.Title(), item.Tags})
	}

	// sort by latest added article first
	sort.Sort(sort.Reverse(ByAdded(items)))
	return items, nil
}

func alreadyHandled(article pocketItem) bool {
	for _, tag := range article.tags {
		if tag.Tag == "remarkable" {
			return true
		}
	}

	return false
}

func registerHandled(article pocketItem) {
	config := getConfig()

	modifyResult := &PocketModifyResult{}

	actions := []PocketModifyActions{
		{"tags_add", article.id, "remarkable"},
		{"archive", article.id, ""},
	}

	body, _ := json.Marshal(PocketModify{
		config.Pocket.ConsumerKey,
		config.Pocket.AccessToken,
		actions,
	})

	req, _ := http.NewRequest("POST", "https://getpocket.com/v3/send", bytes.NewReader(body))
	req.Header.Add("X-Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	if resp.StatusCode != 200 {
		err := fmt.Errorf("got response %d; X-Error=[%s]", resp.StatusCode, resp.Header.Get("X-Error"))
		fmt.Println(err.Error())
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(modifyResult)
	if err != nil {
		fmt.Println(err)
	}
}

func GenerateFiles(maxArticles uint) error {
	fmt.Println("inside generateFiles")
	pocketArticles, err := getPocketItems()
	if err != nil {
		fmt.Println("Could not get pocket articles: ", err)
		return err
	}

	var processed uint = 0
	for _, pocketItem := range pocketArticles {
		if alreadyHandled(pocketItem) {
			fmt.Println("already handled")
			continue
		}

		fileName := getFilename(pocketItem.added, pocketItem.title)
		extension := filepath.Ext(pocketItem.url.String())
		if extension == ".pdf" {
			fileContent := createPDFFileContent(pocketItem.url.String())
			generatePDF(fileName, fileContent)
		} else {
			title, XMLcontent, err := getReadableArticle(pocketItem.url)
			if err != nil {
				fmt.Println(fmt.Sprintf("Could not get readable article: %s (%s)", err, pocketItem.url))
				registerHandled(pocketItem)
				continue
			}
			fileContent := createEpubFileContent(title, XMLcontent)
			generateEpub(fileName, fileContent)
		}

		registerHandled(pocketItem)
		processed++
		fmt.Println(fmt.Sprintf("progress: %d/%d", processed, maxArticles))
		if processed == maxArticles {
			break
		}
	}

	return nil
}
