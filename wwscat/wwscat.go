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
	listenAddr = kingpin.Flag("listen", "Listen to this TCP host:port (instead of stdio)").Default("").OverrideDefaultFromEnvar("WWS_TCP_LISTEN").Short('l').TCP()
	proxyAddr  = kingpin.Flag("proxy", "Proxy to this TCP host:port").Default("").OverrideDefaultFromEnvar("PROXY").Short('p').TCP()
	wsURL      = kingpin.Arg("url", "URL of the websocket server").Required().URL()
	wait       = kingpin.Flag("wait", "Wait for the other side to connect before starting to listen or proxy").Default("false").Short('w').Bool()
)

func main() {
	kingpin.Parse()

	ws := connect((*wsURL).String())
	trapCtrlC(ws)

	if *wait {
		for {
			log.Println("Waiting for both sides to connect before starting to listen or proxy")
			messageType, buf, _ := ws.ReadMessage()
			if messageType == websocket.TextMessage && string(buf) == "WWS_GOTBOTH" {
				break
			} else {
				log.Println("Got ", string(buf))
			}
		}
	}

	var conn net.Conn
	var err error

	ready := make(chan struct{}, 1)

	if *listenAddr != nil && (*listenAddr).Port != 0 {
		log.Println("Setupping TCP listener on ", (*listenAddr).String())
		conn, err = tcpConn(*listenAddr, ready)
	} else if *proxyAddr != nil && (*proxyAddr).Port != 0 {
		log.Println("Setupping TCP proxy to ", (*proxyAddr).String())
		conn, err = NewCOWConn((*proxyAddr).String(), ready)
	} else {
		conn, err = NewStdioConn(ready)
	}
	kingpin.FatalIfError(err, "Couldn't create listener")

	go write(conn, ws, ready)
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

func tcpConn(addr *net.TCPAddr, ready chan<- struct{}) (n net.Conn, err error) {
	l, err := net.Listen("tcp", addr.String())
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	n, err = l.Accept()
	ready <- struct{}{}
	return n, err
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

// stdin/conn -> ws.
func write(conn net.Conn, ws *websocket.Conn, ready <-chan struct{}) {
	_ = <-ready //wait for ready signal before starting read loop
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

// ws -> stdout/conn
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
