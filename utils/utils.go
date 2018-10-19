package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

func Decode64(encoded string) string {
	data, err := base64.StdEncoding.DecodeString(encoded)

	if err != nil {
		log.Printf("base64 decoding of payload failed: %s\n", err)
	}

	return string(data)
}

func Exec(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	return cmd.Run()
}

func ExecDir(dir string, command string, args ...string) (string, string, error) {
	var outb, errb bytes.Buffer

	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()

	return outb.String(), errb.String(), err
}

func DownloadFile(filepath string, url string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
