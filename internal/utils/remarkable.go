package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pdf "github.com/balacode/one-file-pdf"
	"github.com/google/uuid"
)

type Remarkable struct {
	Config *RemarkableConfig
}

type RemarkableConfig struct {
	Service          string
	ReloadUUID       string
	TargetFolderUUID string
}

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

func (r Remarkable) articeFolderPath() string {
	userHomeDir := getUserHomeDir()

	return filepath.Join(userHomeDir, ".local/share/remarkable/xochitl/")
}

func (r Remarkable) folderIsPresent(uuid string) bool {
	folderPath := filepath.Join(r.articeFolderPath(), uuid+".content")
	metadataPath := filepath.Join(r.articeFolderPath(), uuid+".metadata")
	_, err := os.Stat(folderPath)

	if os.IsNotExist(err) {
		return false
	}

	fileContent, _ := os.ReadFile(metadataPath)
	var metadata MetaData
	_ = json.Unmarshal(fileContent, &metadata)
	return !metadata.Deleted
}

// uuid is returned
func (r Remarkable) generateEpub(visibleName string, fileContent []byte) string {

	var lastModified = fmt.Sprintf("%d", time.Now().Unix())

	config := r.Config
	fileUUID := uuid.New().String()

	fileName := filepath.Join(r.articeFolderPath(), fileUUID+".epub")
	writeFile(fileName, fileContent)

	fileContent = r.getDotContentContent("epub")
	fileName = filepath.Join(r.articeFolderPath(), fileUUID+".content")
	writeFile(fileName, fileContent)

	fileContent = r.getMetadataContent(visibleName, config.TargetFolderUUID, "DocumentType", lastModified)
	fileName = filepath.Join(r.articeFolderPath(), fileUUID+".metadata")
	writeFile(fileName, fileContent)

	return fileUUID
}

func (r Remarkable) generatePDF(visibleName string, fileContent []byte) string {

	var lastModified = fmt.Sprintf("%d", time.Now().Unix())

	config := r.Config
	fileUUID := uuid.New().String()

	fileName := filepath.Join(r.articeFolderPath(), fileUUID+".pdf")
	writeFile(fileName, fileContent)

	fileContent = r.getDotContentContent("pdf")
	fileName = filepath.Join(r.articeFolderPath(), fileUUID+".content")
	writeFile(fileName, fileContent)

	fileContent = r.getMetadataContent(visibleName, config.TargetFolderUUID, "DocumentType", lastModified)
	fileName = filepath.Join(r.articeFolderPath(), fileUUID+".metadata")
	writeFile(fileName, fileContent)

	return fileUUID
}

func (r Remarkable) GenerateTargetFolder() {
	config := r.Config
	targetFolderUUID := r.generateTopLevelFolder(config.Service)
	config.TargetFolderUUID = targetFolderUUID
	writeRemarkableConfig(config)
}

func (r Remarkable) GenerateReloadFile() {
	fmt.Println("writing reloadfile")
	var pdfFile = pdf.NewPDF("Letter")

	pdfFile.SetUnits("in").
		SetFont("Helvetica-Bold", 100).
		SetColor("Black")
	pdfFile.SetXY(1.3, 2).
		DrawText("Remove")
	pdfFile.SetXY(3.5, 4).
		DrawText("to")
	pdfFile.SetXY(2.5, 6).
		DrawText("Sync")
	fileContent := pdfFile.Bytes()

	reloadFileUUID := r.generatePDF("remove to sync", fileContent)
	config := r.Config
	config.ReloadUUID = reloadFileUUID
	writeRemarkableConfig(config)
}

func (r Remarkable) generateTopLevelFolder(folderName string) string {
	var lastModified = fmt.Sprintf("%d", time.Now().Unix())
	fileUUID := uuid.New().String()

	fileName := filepath.Join(r.articeFolderPath(), fileUUID+".content")
	writeFile(fileName, []byte("{}"))

	fileContent := r.getMetadataContent(folderName, "", "CollectionType", lastModified)
	fileName = filepath.Join(r.articeFolderPath(), fileUUID+".metadata")
	writeFile(fileName, fileContent)

	return fileUUID
}

func (r Remarkable) getDotContentContent(fileType string) []byte {
	transform := Transform{1, 0, 0, 0, 1, 0, 0, 0, 1}
	docContent := DocumentContent{ExtraMetaData{}, fileType, "", 0, -1, 100, "portrait", 1, 1, transform}
	content, _ := json.Marshal(docContent)
	return content
}

func (r Remarkable) getMetadataContent(visibleName string, parentUUID string, fileType string, lastModified string) []byte {
	metadataContent := MetaData{false, lastModified, false, false, parentUUID, false, false, fileType, 1, visibleName}
	content, _ := json.Marshal(metadataContent)
	return content
}

func (r Remarkable) TargetFolderExists() bool {
	config := r.Config
	folderUUID := config.TargetFolderUUID
	return r.folderIsPresent(folderUUID)
}

// check both if file is present and (metadata deleted=false or file in trash)
func (r Remarkable) pdfIsPresent(uuid string) bool {

	pdfPath := filepath.Join(r.articeFolderPath(), uuid+".pdf")
	metadataPath := filepath.Join(r.articeFolderPath(), uuid+".metadata")
	_, err := os.Stat(pdfPath)

	if os.IsNotExist(err) {
		return false
	}

	fileContent, _ := os.ReadFile(metadataPath)
	var metadata MetaData
	_ = json.Unmarshal(fileContent, &metadata)
	return !metadata.Deleted && metadata.Parent != "trash"
}

func (r Remarkable) ReloadFileExists() bool {
	config := r.Config
	return r.pdfIsPresent(config.ReloadUUID)
}
