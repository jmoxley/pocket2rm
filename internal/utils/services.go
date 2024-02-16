package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/bmaupin/go-epub"
	"github.com/go-shiori/dom"
	"github.com/go-shiori/go-readability"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

func cleanDuplicateAttributes(doc *html.Node, attrName string) string {
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

func createEpubFileContent(title string, content string, author string) []byte {
	e := epub.NewEpub(title)
	e.SetAuthor(author)
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

// generate filename from time added and title
func getFilename(timeAdded time.Time, title string) string {
	// fileType: "epub" or "pdf"
	fileName := fmt.Sprintf("%s :: %s", timeAdded.Format("20060102-1504"), title)
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
		article.Content = cleanDuplicateAttributes(article.Node, "id")
		article.Content = cleanDuplicateAttributes(article.Node, "alt")
	}

	// Include title and source URL in beginning of content
	content := fmt.Sprintf(`<h1> %s </h1>
		<a href="%s">%s</a>
		%s`, article.Title, url.String(), url.String(), article.Content)

	return article.Title, content, nil
}
