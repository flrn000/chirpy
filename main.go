package main

import (
	"log"
	"os"

	"github.com/flrn000/chirpy/internal/server"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	secret := os.Getenv("JWT_SECRET")
	server.Initialize(secret)
}
