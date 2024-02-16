package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type PocketService struct {
	Name   string
	Config PocketConfig
}

type PocketConfig struct {
	ReloadUUID       string            `yaml:"reloadUUID"`
	TargetFolderUUID string            `yaml:"targetFolderUUID"`
	ConsumerKey      string            `yaml:"consumerKey"`
	AccessToken      string            `yaml:"accessToken"`
	RequestParams    map[string]string `yaml:"requestParams"`
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

func (s PocketService) GetRemarkableConfig() *RemarkableConfig {
	return &RemarkableConfig{
		Service:          s.Name,
		ReloadUUID:       s.Config.ReloadUUID,
		TargetFolderUUID: s.Config.TargetFolderUUID,
	}
}

func (s PocketService) getPocketItems() ([]pocketItem, error) {
	// unfortunately cannot use github.com/motemen/go-pocket
	// because of 32bit architecture
	// Item.ItemID in github.com/motemen/go-pocket is int, which cannot store enough
	// therefore the necessary types and functions have been copied and adapted

	config := s.Config

	retrieveResult := &PocketResult{}

	body, _ := json.Marshal(PocketRetrieve{
		config.ConsumerKey,
		config.AccessToken,
		config.RequestParams["count"],
		config.RequestParams["contentType"],
		config.RequestParams["detailType"],
		config.RequestParams["sort"],
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

func (s PocketService) alreadyHandled(article pocketItem) bool {
	for _, tag := range article.tags {
		if tag.Tag == "remarkable" {
			return true
		}
	}

	return false
}

func (s PocketService) registerHandled(article pocketItem) {
	config := s.Config

	modifyResult := &PocketModifyResult{}

	actions := []PocketModifyActions{
		{"tags_add", article.id, "remarkable"},
		{"archive", article.id, ""},
	}

	body, _ := json.Marshal(PocketModify{
		config.ConsumerKey,
		config.AccessToken,
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

func (s PocketService) GenerateFiles(maxArticles uint) error {
	fmt.Println("inside generateFiles (pocket)")
	rm := Remarkable{Config: s.GetRemarkableConfig()}
	pocketArticles, err := s.getPocketItems()
	if err != nil {
		fmt.Println("Could not get pocket articles: ", err)
		return err
	}

	var processed uint = 0
	for _, pocketItem := range pocketArticles {
		if s.alreadyHandled(pocketItem) {
			fmt.Println("already handled")
			continue
		}

		fileName := getFilename(pocketItem.added, pocketItem.title)
		extension := filepath.Ext(pocketItem.url.String())
		if extension == ".pdf" {
			fileContent := createPDFFileContent(pocketItem.url.String())
			rm.generatePDF(fileName, fileContent)
		} else {
			title, XMLcontent, err := getReadableArticle(pocketItem.url)
			if err != nil {
				fmt.Println(fmt.Sprintf("Could not get readable article: %s (%s)", err, pocketItem.url))
				s.registerHandled(pocketItem)
				continue
			}
			fileContent := createEpubFileContent(title, XMLcontent, "pocket2rm")
			rm.generateEpub(fileName, fileContent)
		}

		s.registerHandled(pocketItem)
		processed++
		fmt.Println(fmt.Sprintf("progress: %d/%d", processed, maxArticles))
		if processed == maxArticles {
			break
		}
	}

	return nil
}
