package helpers

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func CheckEnvExists() bool {
	if _, err := os.Stat(".ENV"); err == nil {
		fmt.Println(".ENV found")
		return true
	} else if os.IsNotExist(err) {
		fmt.Println(".ENV not found")
		return false
	} else {
		fmt.Println("error when trying to read .ENV file")
		fmt.Println("-----------------------------------")
		fmt.Println(err)
		fmt.Println("-----------------------------------")
		return false
	}
}

func LoadEnvVariables() (string, string) {

	var token string
	var appToken string

	godotenv.Load(".env")

	if token = os.Getenv("SLACK_AUTH_TOKEN"); len(token) == 0 {
		fmt.Println("Slack Auth Token not found. Halting.")
		os.Exit(1)
	}
	if appToken = os.Getenv("SLACK_APP_TOKEN"); len(appToken) == 0 {
		fmt.Println("Slack App Token not found. Halting.")
		os.Exit(1)
	}

	return token, appToken
}
