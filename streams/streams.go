package streams

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

// Stream is an interface that reads and writes
type Stream interface {
	Read() ([]byte, error)
	Write(buf []byte) error
}

// StreamAdapter connects a pty to a webSocket
type StreamAdapter struct {
	source  Stream
	streams []Stream
}

// NewAdapter takes streams and returns a StreamAdapter
func NewAdapter(source Stream, stms ...Stream) StreamAdapter {
	adapter := StreamAdapter{source, stms}
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
	if size < 1 {
		message := fmt.Sprintf("StreamAdapter requires exactly two streams, has %v.", size)
		return errors.New(message)
	}

	if a.source == nil {
		return errors.New("StreamAdapter requires a source stream")
	}

	var wg sync.WaitGroup

	for _, stream := range a.streams {
		wg.Add(1)
		go func(stream Stream) {
			err := pipeStreams(a.source, stream)
			log.Println(err)
			wg.Done()
		}(stream)

		wg.Add(1)
		go func(stream Stream) {
			wg.Add(1)
			err := pipeStreams(stream, a.source)
			log.Println(err)
			wg.Done()
		}(stream)
	}

	wg.Wait()

	log.Fatalf("Adapter disconnected")

	return nil
}
