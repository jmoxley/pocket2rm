package main

import (
	"fmt"
	u "pocket2rm/internal/utils"
)

func main() {
	fmt.Println("start program")
	var maxFiles uint = 10

	config := u.GetAppConfig()
	svc, _ := u.GetService(config)
	rm := u.Remarkable{Config: svc.GetRemarkableConfig()}

	if rm.ReloadFileExists() {
		fmt.Println("reload file exists")
	} else {
		fmt.Println("no reload file")
		if !rm.TargetFolderExists() {
			fmt.Println("no target folder")
			rm.GenerateTargetFolder()
		}
		rm.GenerateReloadFile()
		_ = svc.GenerateFiles(maxFiles)
	}
}
