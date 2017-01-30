// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

type TCPConnWithStatus struct {
	conn *net.TCPConn
	up   bool
}

var connectorUrl = flag.String("connector", "ws://localhost:8080", "the middle websocket connector")
var channelId = flag.String("channel", "", "the channel ID (guid)")
var remote = flag.String("remote", "localhost:22", "remote host:port to proxy to")

func main() {
	flag.Parse()

	if *channelId == "" {
		log.Println("The channel ID is mandatory. Please set it (-channel={id})")
		return
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})
	toWS := make(chan []byte)
	fromWS := make(chan []byte)
	controlChan := make(chan string)

	tcpAddr, err := net.ResolveTCPAddr("tcp", *remote)
	if err != nil {
		println("ResolveTCPAddr failed:", err.Error())
		os.Exit(1)
	}

	go func() {
		ts := TCPConnWithStatus{conn: nil, up: false}
		for {
			select {

			case message := <-fromWS:
				if !ts.up {
					log.Println("Connecting to", tcpAddr.String())
					ts.conn, err = net.DialTCP("tcp", nil, tcpAddr)
					if err != nil {
						println("Dial failed:", err.Error())
						os.Exit(1)
					}
					ts.up = true
					go handleTCP(&ts, toWS, controlChan)
				}
				if n, err := ts.conn.Write(message); err != nil || n < len(message) {
					if err != nil {
						log.Fatalln("Error while writing to TCP", err)
					}
					if n < len(message) {
						log.Fatalf("Could't write all message; wrote %v / %v\n", n, len(message))
					}
				}
			}
		}
	}()

	u := *connectorUrl + "/ws/proxy/" + *channelId
	log.Printf("connecting to %s", u)

	ws, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		log.Fatal("handshake failed with status ", resp.StatusCode)
	}
	defer ws.Close()

	go read(ws, fromWS, controlChan, done)
	write(ws, toWS, done, interrupt)

	log.Println("Terminating websocket read pump")
}

func read(ws *websocket.Conn, fromWS chan<- []byte, controlChan chan<- string, done chan struct{}) {
	defer close(done)
	for {
		msgType, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}

		switch msgType {
		case websocket.BinaryMessage:
			fromWS <- message
		case websocket.TextMessage:
			controlChan <- string(message)
		}
	}
	log.Println("Terminating websocket read pump")

	return
}

func write(ws *websocket.Conn, toWS <-chan []byte, done chan struct{}, interrupt chan os.Signal) {
	for {
		select {
		case message := <-toWS:
			if err := ws.WriteMessage(websocket.BinaryMessage, message); err != nil {
				log.Fatalln("Error while sending message to ws:", err)
			}
		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			ws.Close()
			return
		}
	}
}

func handleTCP(ts *TCPConnWithStatus, fromTCP chan<- []byte, controlChan <-chan string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}

		fmt.Println("Done handleRequest")
		if ts.conn != nil {
			ts.conn.Close()
			ts.up = false
		}
	}()

	go func() {
		for {
			buf := make([]byte, 1024*1024*1)
			n, err := ts.conn.Read(buf)

			switch err {
			case io.EOF:
				fmt.Println("EOF")
				return

			case nil:
				message := buf[:n]
				fromTCP <- message

			default:
				log.Printf("Receive data failed:%s\n", err)
				return
			}
		}
	}()

	for {
		select {
		case controlMessage := <-controlChan:
			log.Println("Got control message:", controlMessage)
			return
		}
	}
}
