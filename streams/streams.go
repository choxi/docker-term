package streams

import (
	"errors"
	"fmt"
	"log"
)

// Stream is an interface that reads and writes
type Stream interface {
	Read() ([]byte, error)
	Write(buf []byte) error
}

// StreamAdapter connects a pty to a webSocket
type StreamAdapter struct {
	streams []Stream
}

// NewAdapter takes streams and returns a StreamAdapter
func NewAdapter(stms ...Stream) StreamAdapter {
	adapter := StreamAdapter{}
	adapter.streams = stms
	return adapter
}

func pipeStreams(writer Stream, reader Stream) error {
	var (
		buf []byte
		err error
	)

	// TODO: more graceful exit on socket close / process exit
	for {
		if buf, err = writer.Read(); err != nil {
			return err
		}

		if err = reader.Write(buf); err != nil {
			return err
		}
	}
}

// Connect takes the adapters streams and connects their reads and writes
// It currently only supports two streams
func (a *StreamAdapter) Connect() error {
	// TODO: check for errors, return 500 on fail

	// copy everything from the pty master to the websocket
	// using base64 encoding for now due to limitations in term.js

	size := len(a.streams)
	if size != 2 {
		message := fmt.Sprintf("StreamAdapter requires exactly two streams, has %v.", size)
		return errors.New(message)
	}

	streamA := a.streams[0]
	streamB := a.streams[1]

	go func() {
		err := pipeStreams(streamA, streamB)
		log.Println(err)
	}()
	// read from the web socket, copying to the pty master
	// messages are expected to be text and base64 encoded

	err := pipeStreams(streamB, streamA)
	log.Println(err)
	return nil
}
