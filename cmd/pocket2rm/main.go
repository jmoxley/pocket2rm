package main

import (
	"fmt"
	u "pocket2rm/internal/utils"
)

func main() {
	fmt.Println("start program")
	var maxFiles uint = 10
	if u.ReloadFileExists() {
		fmt.Println("reload file exists")
	} else {
		fmt.Println("no reload file")
		if !u.PocketFolderExists() {
			fmt.Println("no pocket folder")
			u.GeneratePocketFolder()
		}
		u.GenerateReloadFile()
		u.GenerateFiles(maxFiles)
	}
}
