package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	pdf "github.com/balacode/one-file-pdf"
	"github.com/google/uuid"
)

type DocumentContent struct {
	ExtraMetadata  ExtraMetaData `json:"extraMetadata"`
	FileType       string        `json:"fileType"`
	FontName       string        `json:"fontName"`
	LastOpenedPage int           `json:"lastOpenedPage"`
	LineHeight     int           `json:"lineHeight"`
	Margins        int           `json:"margins"`
	Orientation    string        `json:"orientation"`
	PageCount      int           `json:"pageCount"`
	TextScale      int           `json:"textScale"`
	Transform      Transform     `json:"transform"`
}

type ExtraMetaData struct {
}

type MetaData struct {
	Deleted          bool   `json:"deleted"`
	LastModified     string `json:"lastModified"`
	Metadatamodified bool   `json:"metadatamodified"`
	Modified         bool   `json:"modified"`
	Parent           string `json:"parent"` //uuid or "trash"
	Pinned           bool   `json:"pinned"`
	Synced           bool   `json:"synced"`
	Type             string `json:"type"`
	Version          int    `json:"version"`
	VisibleName      string `json:"visibleName"`
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

func articeFolderPath() string {
	userHomeDir := getUserHomeDir()

	return filepath.Join(userHomeDir, ".local/share/remarkable/xochitl/")
}

func folderIsPresent(uuid string) bool {
	folderPath := filepath.Join(articeFolderPath(), uuid+".content")
	metadataPath := filepath.Join(articeFolderPath(), uuid+".metadata")
	_, err := os.Stat(folderPath)

	if os.IsNotExist(err) {
		return false
	}

	fileContent, _ := ioutil.ReadFile(metadataPath)
	var metadata MetaData
	json.Unmarshal(fileContent, &metadata)
	return !metadata.Deleted
}

// uuid is returned
func generateEpub(visibleName string, fileContent []byte) string {

	var lastModified = fmt.Sprintf("%d", time.Now().Unix())

	config := getConfig()
	fileUUID := uuid.New().String()

	fileName := filepath.Join(articeFolderPath(), fileUUID+".epub")
	writeFile(fileName, fileContent)

	fileContent = getDotContentContent("epub")
	fileName = filepath.Join(articeFolderPath(), fileUUID+".content")
	writeFile(fileName, fileContent)

	fileContent = getMetadataContent(visibleName, config.TargetFolderUUID, "DocumentType", lastModified)
	fileName = filepath.Join(articeFolderPath(), fileUUID+".metadata")
	writeFile(fileName, fileContent)

	return fileUUID
}

func generatePDF(visibleName string, fileContent []byte) string {

	var lastModified = fmt.Sprintf("%d", time.Now().Unix())

	config := getConfig()
	fileUUID := uuid.New().String()

	fileName := filepath.Join(articeFolderPath(), fileUUID+".pdf")
	writeFile(fileName, fileContent)

	fileContent = getDotContentContent("pdf")
	fileName = filepath.Join(articeFolderPath(), fileUUID+".content")
	writeFile(fileName, fileContent)

	fileContent = getMetadataContent(visibleName, config.TargetFolderUUID, "DocumentType", lastModified)
	fileName = filepath.Join(articeFolderPath(), fileUUID+".metadata")
	writeFile(fileName, fileContent)

	return fileUUID
}

func GenerateTargetFolder() {
	config := getConfig()
	targetFolderUUID := generateTopLevelFolder(config.Service)
	config.TargetFolderUUID = targetFolderUUID
	writeConfig(config)
}

func GenerateReloadFile() {
	fmt.Println("writing reloadfile")
	var pdf = pdf.NewPDF("A4")

	pdf.SetUnits("cm").
		SetFont("Helvetica-Bold", 100).
		SetColor("Black")
	pdf.SetXY(3.5, 5).
		DrawText("Remove")
	pdf.SetXY(9, 10).
		DrawText("to")
	pdf.SetXY(6.5, 15).
		DrawText("Sync")
	fileContent := pdf.Bytes()

	reloadFileUUID := generatePDF("remove to sync", fileContent)
	config := getConfig()
	config.ReloadUUID = reloadFileUUID
	writeConfig(config)
}

func generateTopLevelFolder(folderName string) string {
	var lastModified = fmt.Sprintf("%d", time.Now().Unix())
	fileUUID := uuid.New().String()

	fileName := filepath.Join(articeFolderPath(), fileUUID+".content")
	writeFile(fileName, []byte("{}"))

	fileContent := getMetadataContent(folderName, "", "CollectionType", lastModified)
	fileName = filepath.Join(articeFolderPath(), fileUUID+".metadata")
	writeFile(fileName, fileContent)

	return fileUUID
}

func getDotContentContent(fileType string) []byte {
	transform := Transform{1, 0, 0, 0, 1, 0, 0, 0, 1}
	docContent := DocumentContent{ExtraMetaData{}, fileType, "", 0, -1, 100, "portrait", 1, 1, transform}
	content, _ := json.Marshal(docContent)
	return content
}

func getMetadataContent(visibleName string, parentUUID string, fileType string, lastModified string) []byte {
	metadataContent := MetaData{false, lastModified, false, false, parentUUID, false, false, fileType, 1, visibleName}
	content, _ := json.Marshal(metadataContent)
	return content
}

func TargetFolderExists() bool {
	config := getConfig()
	folderUUID := config.TargetFolderUUID
	return folderIsPresent(folderUUID)
}

// check both if file is present and (metadata deleted=false or file in trash)
func pdfIsPresent(uuid string) bool {

	pdfPath := filepath.Join(articeFolderPath(), uuid+".pdf")
	metadaPath := filepath.Join(articeFolderPath(), uuid+".metadata")
	_, err := os.Stat(pdfPath)

	if os.IsNotExist(err) {
		return false
	}

	fileContent, _ := ioutil.ReadFile(metadaPath)
	var metadata MetaData
	json.Unmarshal(fileContent, &metadata)
	return !metadata.Deleted && metadata.Parent != "trash"
}

func ReloadFileExists() bool {
	config := getConfig()
	return pdfIsPresent(config.ReloadUUID)
}
