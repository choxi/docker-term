package ws

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1,
	WriteBufferSize: 1,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type key int

const wsKey key = 0

// WS holds a websocket connection
type WS struct {
	connection *websocket.Conn
}

func (ws *WS) Write(buf []byte) error {
	return ws.connection.WriteMessage(websocket.TextMessage, buf)
}

// ReadMessage returns the next bytes written to the connection
func (ws *WS) Read() ([]byte, error) {
	mt, payload, err := ws.connection.ReadMessage()

	if err != nil {
		if err != io.EOF {
			return nil, err
		}
	}

	if mt != websocket.TextMessage {
		return nil, errors.New("Can only decode text messages")
	}

	buf, err := base64.StdEncoding.DecodeString(string(payload))

	if err != nil {
		return nil, err
	}

	return buf, nil
}

// Middleware creates a websocket connection and adds it to the request context
// defer conn.Close()
func Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("start wsMiddlewareOne")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatalf("Websocket upgrade failed: %s\n", err)
		}

		ws := WS{conn}
		ctx := context.WithValue(r.Context(), wsKey, ws)
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Println("end wsMiddlewareOne")
	}
}

// GetWS takes a context and returns its WS connection
func FromContext(ctx context.Context) WS {
	return ctx.Value(wsKey).(WS)
}
