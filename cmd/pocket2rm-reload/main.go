package main

import (
	"fmt"
	"os/exec"
	"time"

	u "pocket2rm/internal/utils"
)

func startPocket2rm() {
	cmd := exec.Command("systemctl", "restart", "pocket2rm")
	cmd.Run()
}

func main() {
	fmt.Println("start program")

	config := u.GetAppConfig()
	svc, _ := u.GetService(config)
	rm := u.Remarkable{Config: svc.GetRemarkableConfig()}

	for {
		fmt.Println("sleep for 10 secs")
		time.Sleep(10 * time.Second)
		if rm.ReloadFileExists() {
			fmt.Println("reload file exists")
		} else {
			fmt.Println("no reload file, starting pocket2rm")
			startPocket2rm()
		}
	}
}
