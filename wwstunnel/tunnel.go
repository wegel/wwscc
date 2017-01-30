// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "the middle websocket connector")
var channelId = flag.String("channel", "", "the channel ID (guid)")
var port = flag.String("port", "45352", "the port to listen to")

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

	l := setupTCPListener()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				fmt.Println("Error accepting: ", err.Error())
				os.Exit(1)
			}
			fmt.Printf("Got a TCP connection on port %s", *port)
			go handleTCP(conn, toWS, fromWS, controlChan)
		}
	}()

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws/tunnel/" + *channelId}
	log.Printf("connecting to %s", u.String())

	ws, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("handshake failed with status ", resp.StatusCode)
	}
	defer ws.Close()

	go func() {
		defer close(done)
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			//log.Println("tunnel recv from ws:", truncateString(string(message), 20))
			fromWS <- message
		}
	}()

	for {
		select {
		case message := <-toWS:
			log.Println("tunnel send to ws:", truncateString(string(message), 20))
			ws.WriteMessage(websocket.BinaryMessage, message)
		case message := <-controlChan:
			log.Println("tunnel send control to ws:", truncateString(string(message), 20))
			ws.WriteMessage(websocket.TextMessage, []byte("close"))
			//os.Exit(0)
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

func handleTCP(tcp net.Conn, fromTCP chan<- []byte, toTCP <-chan []byte, controlChan chan<- string) {
	defer func() {
		fmt.Println("Done handleRequest")
		tcp.Close()
	}()

	go func() {
		for {
			select {
			case message := <-toTCP:
				//log.Println("tunnel send to tcp:", truncateString(string(message), 20))
				tcp.Write(message)
			}
		}
	}()

	buf := make([]byte, 1048576)
	for {
		n, err := tcp.Read(buf)

		switch err {
		case io.EOF:
			fmt.Println("EOF")
			controlChan <- "close"
			return

		case nil:
			//log.Printf("tunnel recv from tcp(%v):%s\n", n, truncateString(string(buf[:n]), 20))
			fromTCP <- buf[:n]

		default:
			log.Fatalf("Receive data failed:%s", err)
			return
		}
		//fmt.Println("5")
	}
}

func setupTCPListener() net.Listener {
	log.Println("Setupping TCP listener on port", *port)
	l, err := net.Listen("tcp", "localhost:"+*port)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	fmt.Println("Listening on " + string(*port))
	return l
}

func getFreePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for !utf8.ValidString(s[:n]) {
		n--
	}
	return s[:n]
}
