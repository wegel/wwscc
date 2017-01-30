// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s [URL]", os.Args[0])
		os.Exit(1)
	}

	ws := connect(os.Args[1])
	trapCtrlC(ws)

	go write(ws)
	read(ws)
}

func connect(addr string) *websocket.Conn {
	log.Printf("connecting to %s...", addr)
	ws, resp, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		log.Fatal("handshake failed with status ", resp.StatusCode)
	}
	log.Print("ready, exit with CTRL+C.")
	return ws
}

func trapCtrlC(ws *websocket.Conn) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
			fmt.Println("\nexiting")
			ws.Close()
			os.Exit(0)
		}
	}()
}

// stdin -> ws.
func write(ws *websocket.Conn) {
	//below WriteMessage returns when the data has been flushed, so safe to reuse buffer
	buf := make([]byte, 64*1024) // pipe buffer is usually 64kb
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
			log.Fatalln("Error while writing to ws:", err)
		}
	}
}

// ws -> stdout
func read(ws *websocket.Conn) {
	for {
		messageType, buf, err := ws.ReadMessage()
		if err != nil {
			return
		}
		if messageType == websocket.BinaryMessage {
			if n, err := os.Stdout.Write(buf); err != nil || n < len(buf) {
				log.Fatalln("Error writing to websocket from stdin:", err)
			}
		}
	}
}
