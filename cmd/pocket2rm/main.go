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
		if !u.TargetFolderExists() {
			fmt.Println("no target folder")
			u.GenerateTargetFolder()
		}
		u.GenerateReloadFile()
		u.GenerateFiles(maxFiles)
	}
}
