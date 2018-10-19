package main

import (
	"Dre/server"
	"os"
)

func main() {
	dir, _ := os.Getwd()
	server := server.New(dir)
	server.Start("3000")
}
