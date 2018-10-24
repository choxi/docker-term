package docker

import (
	"dre/utils"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"

	pseudoterm "github.com/kr/pty"
	uuid "github.com/satori/go.uuid"
)

// Container is a Docker container
type Container struct {
	ID      uuid.UUID
	imageID uuid.UUID
	pty     *Pty
	OnStart func() error
	OnStop  func() error
}

// Pty is a pty connection to a command
type Pty struct {
	Cmd  *exec.Cmd // pty builds on os.exec
	Conn *os.File  // a pty is simply an os.File
}

// CreateContainer takes a source URL for a repo with a Dockerfile,
// builds an image for it, and returns a Container for that image.
func CreateContainer(containerID uuid.UUID, sourceURL string) (Container, error) {
	var err error

	imageID, _ := uuid.NewV4()

	downloadPath := fmt.Sprintf("./tmp/containers/%s/", imageID.String())
	repoPath := downloadPath + "repo"
	tarTarget := "tar_repo.tgz"
	os.MkdirAll(repoPath, os.ModePerm)

	log.Println("Downloading repo...")
	if err = utils.DownloadFile(downloadPath+tarTarget, sourceURL); err != nil {
		return Container{}, err
	}

	log.Println("Unarchiving repo...")
	if _, _, err = utils.ExecDir(downloadPath, "tar", "-C", "./repo", "-xzf", tarTarget, "--strip-components=1"); err != nil {
		return Container{}, err
	}

	log.Println("Building image...")
	if _, _, err = utils.ExecDir(repoPath, "docker", "build", "-t", imageID.String(), "."); err != nil {
		return Container{}, err
	}

	return Container{ID: containerID, imageID: imageID}, nil
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

	pty.Cmd = exec.Command("docker", "run", "--name", c.ID.String(), "-it", c.imageID.String(), command)
	if pty.Conn, err = pseudoterm.Start(pty.Cmd); err != nil {
		return Pty{}, utils.Error(err, "docker: pty not started")
	}

	if c.OnStart != nil {
		if err = c.OnStart(); err != nil {
			pty.Stop()
			return Pty{}, utils.Error(err, "docker: onstart failed")
		}
	}

	return pty, nil
}

// Stop kills the container and cleans up its volume and image
func (c *Container) Stop() error {
	var err error

	// if err = p.Cmd.Process.Kill(); err != nil {
	// 	return utils.Error(err, "docker: could not kill process")
	// }

	if err = utils.Exec("docker", "kill", c.ID.String()); err != nil {
		return utils.Error(err, "docker: container not stopped")
	}

	if err = utils.Exec("docker", "rm", "-v", c.ID.String()); err != nil {
		return utils.Error(err, "docker: container not removed")
	}

	if c.OnStop != nil {
		if err = c.OnStop(); err != nil {
			return utils.Error(err, "")
		}
	}

	return nil
}

// Stop closes the pty connection
func (p *Pty) Stop() error {
	var err error

	if err = p.Conn.Close(); err != nil {
		return utils.Error(err, "docker: pty not closed")
	}

	// if err = p.Cmd.Wait(); err != nil {
	// 	if err.Error() != "signal: killed" {
	// 		return utils.Error(err, "docker: bad kill")
	// 	}
	// }

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
