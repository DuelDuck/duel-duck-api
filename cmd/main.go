package main

import (
	"log"

	"github.com/joho/godotenv"
	"gitlab.com/duel-duck/duel-duck-api/app"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalln("no .env file found")
	}
}

// @title			Duel Duck API
// @version		1.0
// @description	This is a swagger specification for a Duel Duck back-end.
func main() {
	app.Build().Run()
}
