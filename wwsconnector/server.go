// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	channels       map[uuid.UUID]*Channel
	createChannel  chan *Channel
	registerProxy  chan *Register
	registerTunnel chan *Register
}

func newHub() *Hub {
	return &Hub{
		channels:       make(map[uuid.UUID]*Channel),
		createChannel:  make(chan *Channel),
		registerProxy:  make(chan *Register),
		registerTunnel: make(chan *Register),
	}
}

type Channel struct {
	proxy  *Client //the proxy is on the network that we can't reach
	tunnel *Client //the tunnel typically runs on our local computer
	id     uuid.UUID
	hub    *Hub
}

type Register struct {
	remote *Client
	id     uuid.UUID
	hub    *Hub
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	control   chan string
	otherSide *Client
}

func (h *Hub) handleMessages() {
	log.Println("Waiting for messages on channels")
	for {
		select {
		case channel := <-h.createChannel:
			log.Printf("Creating new channel ID: %v", channel.id.String())
			h.channels[channel.id] = channel

		//the proxy is on the network that we can't reach
		case info := <-h.registerProxy:
			log.Printf("Registering proxy for channel ID: %v", info.id.String())
			h.channels[info.id].proxy = info.remote
			if h.channels[info.id].tunnel != nil {
				log.Printf("Got both sides for channel ID: %v", info.id.String())
				h.channels[info.id].tunnel.otherSide = h.channels[info.id].proxy
				h.channels[info.id].proxy.otherSide = h.channels[info.id].tunnel
			}

		//the tunnel typically runs on our local computer
		case info := <-h.registerTunnel:
			log.Printf("Registering tunnel for channel ID: %v", info.id.String())
			h.channels[info.id].tunnel = info.remote
			if h.channels[info.id].proxy != nil {
				log.Printf("Got both sides for channel ID: %v", info.id.String())
				h.channels[info.id].tunnel.otherSide = h.channels[info.id].proxy
				h.channels[info.id].proxy.otherSide = h.channels[info.id].tunnel
			}
		}
	}
}

func setRemote(hub *Hub, w http.ResponseWriter, r *http.Request, p httprouter.Params, register chan<- *Register, remoteType string) {
	id, _ := uuid.Parse(p.ByName("id"))

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer ws.Close()

	client := &Client{hub: hub, conn: ws, send: make(chan []byte, 2048), control: make(chan string, 256)}

	info := &Register{hub: hub, id: id, remote: client}

	register <- info

	go read(ws, client, remoteType)
	write(ws, client, remoteType)
}

func read(ws *websocket.Conn, client *Client, remoteType string) {
	for {
		for client.otherSide == nil || client.otherSide.send == nil {
			time.Sleep(time.Millisecond * 50)
		}

		msgType, message, err := ws.ReadMessage()
		if err != nil {
			log.Printf("%s read error: %v\n", remoteType, err)
			break
		} else {
			switch msgType {
			case websocket.BinaryMessage:
				if client.otherSide != nil {
					client.otherSide.send <- message
				}
			case websocket.TextMessage:
				if client.otherSide != nil {
					log.Println("Sending control message")
					client.otherSide.control <- "close"
				}
			}

		}
	}
}

func write(ws *websocket.Conn, client *Client, remoteType string) {
	for {
		select {
		case message := <-client.send:
			err := ws.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				log.Printf("%s write error: %v\n", remoteType, err)
				break
			}
		case controlMessage := <-client.control:
			ws.WriteMessage(websocket.TextMessage, []byte(controlMessage))
		}
	}
}

func createChannel(hub *Hub, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	log.Printf("Creating new channel")
	id := uuid.New()

	channel := &Channel{hub: hub, id: id}
	channel.hub.createChannel <- channel

	w.Write([]byte(id.String()))
}

func serveFile(hub *Hub, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	log.Printf("Creating new channel")
	id := uuid.New()

	channel := &Channel{hub: hub, id: id}
	channel.hub.createChannel <- channel

	w.Write([]byte(id.String()))
}

func sshClient(hub *Hub, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	log.Printf("Connecting ssh client")
	id, _ := uuid.Parse(p.ByName("id"))

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer ws.Close()

	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	username := r.URL.Query().Get("username")

	log.Printf("SSH client parameters: user: %s cols: %v rows: %v\n", username, cols, rows)
	sshShell(ws, hub.channels[id].proxy, username, cols, rows)
}

func updateSSH(hub *Hub, w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	log.Printf("Updating SSH")
}

func main() {
	flag.Parse()
	hub := newHub()

	router := httprouter.New()
	router.NotFound = http.FileServer(http.Dir("public"))

	router.GET("/create", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) { createChannel(hub, w, r, p) })

	router.GET("/ws/proxy/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		setRemote(hub, w, r, p, hub.registerProxy, "proxy")
	})
	router.GET("/ws/tunnel/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		setRemote(hub, w, r, p, hub.registerTunnel, "tunnel")
	})

	router.GET("/ws/ssh/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		sshClient(hub, w, r, p)
	})

	router.POST("/ws/ssh/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		updateSSH(hub, w, r, p)
	})

	go hub.handleMessages()
	log.Fatal(http.ListenAndServe(*addr, router))
}
