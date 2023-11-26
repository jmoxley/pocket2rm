package main

import (
	"fmt"
	u "pocket2rm/internal/utils"
)

func main() {
	fmt.Println("start program")
	var maxFiles uint = 10

	config := u.GetConfig()
	svc, _ := u.GetService(config)

	if u.ReloadFileExists() {
		fmt.Println("reload file exists")
	} else {
		fmt.Println("no reload file")
		if !u.TargetFolderExists() {
			fmt.Println("no target folder")
			u.GenerateTargetFolder()
		}
		u.GenerateReloadFile()
		_ = svc.GenerateFiles(maxFiles)
	}
}
