// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"net"

	"github.com/gorilla/websocket"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	tcpPort = kingpin.Flag("listen", "Listen to this TCP host:port instead of stdio").Default("").OverrideDefaultFromEnvar("WWS_TCP_LISTEN").Short('l').TCP()
	wsURL   = kingpin.Arg("URL", "URL of the websocket server").Required().URL()
)

func main() {
	kingpin.Parse()

	ws := connect((*wsURL).String())
	trapCtrlC(ws)

	var conn net.Conn
	var err error

	if *tcpPort != nil && (*tcpPort).Port != 0 {
		log.Println("Setupping TCP listener on ", (*tcpPort).String())
		conn, err = tcpConn(*tcpPort)
	} else {
		conn, err = stdioConn()
	}
	kingpin.FatalIfError(err, "Couldn't create listener")

	go write(conn, ws)
	read(conn, ws)
}

func connect(url string) *websocket.Conn {
	log.Printf("connecting to %s...", url)
	ws, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("handshake failed with status ", resp.StatusCode)
	}
	log.Print("ready, exit with CTRL+C.")
	return ws
}

func tcpConn(addr *net.TCPAddr) (net.Conn, error) {
	l, err := net.Listen("tcp", addr.String())
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	return l.Accept()
}

func stdioConn() (net.Conn, error) {
	return NewConn()
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
func write(conn net.Conn, ws *websocket.Conn) {
	//below WriteMessage returns when the data has been flushed, so safe to reuse buffer
	buf := make([]byte, 64*1024) // pipe buffer is usually 64kb
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Fatalln(err)
		}
		if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
			log.Fatalln("Error while writing to ws:", err)
		}
	}
}

// ws -> stdout
func read(conn net.Conn, ws *websocket.Conn) {
	for {
		messageType, buf, err := ws.ReadMessage()
		if err != nil {
			return
		}
		if messageType == websocket.BinaryMessage {
			if n, err := conn.Write(buf); err != nil || n < len(buf) {
				log.Fatalln("Error writing to websocket from stdin:", err)
			}
		}
	}
}
