package main

import (
	"Dre/server"
	"os"
)

func main() {
	dir, _ := os.Getwd()
	server := server.New()
	server.Start(dir, "3000")
}
