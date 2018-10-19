package main

import (
	"Dre/utils"
	"Dre/ws"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/kr/pty"
	"github.com/satori/go.uuid"
)

var addrFlag, cmdFlag, staticFlag string

type wsPty struct {
	Cmd *exec.Cmd // pty builds on os.exec
	Pty *os.File  // a pty is simply an os.File
}

func (wp *wsPty) Start(cmd string, args ...string) {
	var err error
	wp.Cmd = exec.Command(cmd, args...)
	wp.Pty, err = pty.Start(wp.Cmd)
	if err != nil {
		log.Fatalf("Failed to start command: %s\n", err)
	}
}

func (wp *wsPty) Stop() {
	wp.Pty.Close()
	wp.Cmd.Wait()
}

func ptyHandler(w http.ResponseWriter, r *http.Request) {
	var (
		stdout string
		stderr string
		err    error
	)

	webSocket := ws.GetWS(r.Context())

	sourceURL := utils.Decode64(r.URL.Query()["source_url"][0])
	imageID, _ := uuid.NewV4()
	downloadPath := "./tmp/containers/1/"
	repoPath := downloadPath + "repo"
	tarTarget := "tar_repo.tgz"
	os.MkdirAll(repoPath, os.ModePerm)

	if err = utils.DownloadFile(downloadPath+tarTarget, sourceURL); err != nil {
		panic(err)
	}

	if _, _, err = utils.ExecDir(downloadPath, "tar", "-C", "./repo", "-xzf", "tar_repo.tgz", "--strip-components=1"); err != nil {
		panic(err)
	}

	if stdout, stderr, err = utils.ExecDir(repoPath, "docker", "build", "-t", imageID.String(), "."); err != nil {
		panic(err)
	}

	fmt.Println(stdout)
	fmt.Println(stderr)

	wp := wsPty{}
	// TODO: check for errors, return 500 on fail
	wp.Start("docker", "run", "-it", imageID.String(), "/bin/bash")

	// copy everything from the pty master to the websocket
	// using base64 encoding for now due to limitations in term.js
	go func() {
		buf := make([]byte, 128)
		// TODO: more graceful exit on socket close / process exit
		for {
			n, err := wp.Pty.Read(buf)
			if err != nil {
				log.Printf("Failed to read from pty master: %s", err)
				return
			}

			out := make([]byte, base64.StdEncoding.EncodedLen(n))
			base64.StdEncoding.Encode(out, buf[0:n])

			err = webSocket.WriteMessage(out)

			if err != nil {
				log.Printf("Failed to send %d bytes on websocket: %s", n, err)
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

		wp.Pty.Write(buf)
	}

	wp.Stop()
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
