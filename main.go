package main

import (
	"Dre/docker"
	"Dre/utils"
	"Dre/ws"
	"flag"
	"log"
	"net/http"
	"os"
)

var addrFlag, cmdFlag, staticFlag string

func ptyHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err error
		pty docker.Pty
	)

	webSocket := ws.GetWS(r.Context())
	sourceURL := utils.Decode64(r.URL.Query()["source_url"][0])
	container := docker.CreateContainer(sourceURL)
	pty, err = container.Bash()

	if err != nil {
		return
	}

	// TODO: check for errors, return 500 on fail

	// copy everything from the pty master to the websocket
	// using base64 encoding for now due to limitations in term.js
	go func() {
		// TODO: more graceful exit on socket close / process exit
		for {
			var out []byte

			if out, err = pty.Read(); err != nil {
				return
			}

			if err = webSocket.WriteMessage(out); err != nil {
				return
			}
		}
	}()

	// read from the web socket, copying to the pty master
	// messages are expected to be text and base64 encoded
	for {
		var (
			err error
			buf []byte
		)

		if buf, err = webSocket.ReadMessage(); err != nil {
			return
		}

		if err = pty.Write(buf); err != nil {
			return
		}
	}

	pty.Stop()
	container.Stop()
}

func init() {
	cwd, _ := os.Getwd()
	flag.StringVar(&addrFlag, "addr", ":9000", "IP:PORT or :PORT address to listen on")
	flag.StringVar(&cmdFlag, "cmd", "/bin/bash", "command to execute on slave side of the pty")
	flag.StringVar(&staticFlag, "static", cwd, "path to static content")
	// TODO: make sure paths exist and have correct permissions
}

func main() {
	flag.Parse()

	finalHandler := http.HandlerFunc(ptyHandler)
	http.Handle("/pty", ws.Middleware(finalHandler))

	// serve html & javascript
	http.Handle("/", http.FileServer(http.Dir(staticFlag)))

	err := http.ListenAndServe(addrFlag, nil)
	if err != nil {
		log.Fatalf("net.http could not listen on address '%s': %s\n", addrFlag, err)
	}
}
