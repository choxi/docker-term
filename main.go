package main

import (
	"dre/server"
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		port *int
		dir  string
		err  error
		api  server.Server
	)

	port = flag.Int("port", 3000, "port number to listen on")

	flag.Parse()

	if dir, err = os.Getwd(); err != nil {
		fmt.Println("Could not get working directory")
		return
	}

	api = server.New()
	api.Start(dir, *port)
}
