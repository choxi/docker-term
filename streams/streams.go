package streams

import (
	"dre/utils"
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

// Mux takes a writer stream and connects its outputs to multiple readers
type Mux struct {
	writer  Stream
	readers []Stream
}

// Adapter connects a pty to a webSocket
type Adapter struct {
	source       Stream
	streams      []Stream
	mux          *Mux
	OnDisconnect func() error
}

// NewAdapter takes streams and returns a Adapter
func NewAdapter(source Stream, stms ...Stream) Adapter {
	adapter := Adapter{source: source, streams: stms}
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

func (m *Mux) Connect() error {
	var (
		buf []byte
		err error
	)

	for {
		if buf, err = m.writer.Read(); err != nil {
			return err
		}

		for _, reader := range m.readers {
			if err = reader.Write(buf); err != nil {
				return err
			}
		}
	}
}

// Connect takes the adapters streams and connects their reads and writes
// It currently only supports two streams
func (a *Adapter) Connect() error {
	// TODO: check for errors, return 500 on fail

	// copy everything from the pty master to the websocket
	// using base64 encoding for now due to limitations in term.js

	size := len(a.streams)
	if size < 1 {
		message := fmt.Sprintf("Adapter requires exactly two streams, has %v.", size)
		return errors.New(message)
	}

	if a.source == nil {
		return errors.New("Adapter requires a source stream")
	}

	var wg sync.WaitGroup

	a.mux = &Mux{}
	a.mux.writer = a.source
	a.mux.readers = a.streams
	go a.mux.Connect()

	for _, str := range a.streams {
		wg.Add(1)
		go func(s Stream) {
			if err := pipeStreams(s, a.source); err != nil {
				log.Println(err)
			}

			wg.Done()
		}(str)
	}

	wg.Wait()

	if a.OnDisconnect != nil {
		if err := a.OnDisconnect(); err != nil {
			return utils.Error(err, "streams: ondisconnect failed")
		}
	}

	return nil
}

// AddStream adds a stream to the adapter and connects it to the source
func (a *Adapter) AddStream(str Stream) {
	a.mux.readers = append(a.mux.readers, str)
	err := pipeStreams(str, a.source)
	log.Println(err)
}
