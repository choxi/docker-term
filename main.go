package main

import (
	"Dre/utils"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
	"github.com/kr/pty"
	"github.com/satori/go.uuid"
)

var addrFlag, cmdFlag, staticFlag string

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1,
	WriteBufferSize: 1,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatalf("Websocket upgrade failed: %s\n", err)
	}
	defer conn.Close()

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

			err = conn.WriteMessage(websocket.TextMessage, out)

			if err != nil {
				log.Printf("Failed to send %d bytes on websocket: %s", n, err)
				return
			}
		}
	}()

	// read from the web socket, copying to the pty master
	// messages are expected to be text and base64 encoded
	for {
		mt, payload, err := conn.ReadMessage()
		if err != nil {
			if err != io.EOF {
				log.Printf("conn.ReadMessage failed: %s\n", err)
				return
			}
		}

		switch mt {
		case websocket.BinaryMessage:
			log.Printf("Ignoring binary message: %q\n", payload)
		case websocket.TextMessage:
			buf := make([]byte, base64.StdEncoding.DecodedLen(len(payload)))
			_, err := base64.StdEncoding.Decode(buf, payload)
			if err != nil {
				log.Printf("base64 decoding of payload failed: %s\n", err)
			}

			wp.Pty.Write(buf)
		default:
			log.Printf("Invalid message type %d\n", mt)
			return
		}
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

	http.HandleFunc("/pty", ptyHandler)

	// serve html & javascript
	http.Handle("/", http.FileServer(http.Dir(staticFlag)))

	err := http.ListenAndServe(addrFlag, nil)
	if err != nil {
		log.Fatalf("net.http could not listen on address '%s': %s\n", addrFlag, err)
	}
}
