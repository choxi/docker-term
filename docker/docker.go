package docker

import (
	"Dre/utils"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"

	pseudoterm "github.com/kr/pty"
	uuid "github.com/satori/go.uuid"
)

type Container struct {
	imageID uuid.UUID
	pty     *Pty
}

type Pty struct {
	Cmd  *exec.Cmd // pty builds on os.exec
	Conn *os.File  // a pty is simply an os.File
}

func CreateContainer(sourceURL string) Container {
	var (
		stdout string
		stderr string
		err    error
	)

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

	return Container{imageID, nil}
}

// Bash runs /bin/bash in the container and returns a pty connection
func (c *Container) Bash() (Pty, error) {
	return c.Run("/bin/bash")
}

// Run runs a command in the container and returns a pty connection
func (c *Container) Run(command string) (Pty, error) {
	var (
		err error
		pty Pty
	)

	pty.Cmd = exec.Command("docker", "run", "-it", c.imageID.String(), command)
	pty.Conn, err = pseudoterm.Start(pty.Cmd)

	if err != nil {
		return Pty{}, err
	}

	return pty, nil
}

// Stop kills the container and cleans up its volume and image
func (c *Container) Stop() error {
	return nil
}

// Stop closes the pty connection
func (p *Pty) Stop() error {
	var err error

	if err = p.Conn.Close(); err != nil {
		return err
	}

	if err = p.Cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func (p *Pty) Write(buf []byte) error {
	_, err := p.Conn.Write(buf)
	return err
}

func (p *Pty) Read() ([]byte, error) {
	buf := make([]byte, 128)
	n, err := p.Conn.Read(buf)

	if err != nil {
		return nil, err
	}

	out := make([]byte, base64.StdEncoding.EncodedLen(n))
	base64.StdEncoding.Encode(out, buf[0:n])

	if err != nil {
		return nil, err
	}

	return out, nil
}
