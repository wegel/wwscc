// Author: Simon Labrecque <simon@wegel.ca>

package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

var addr = flag.String("addr", ":8080", "http service address")

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 5) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 128,
	WriteBufferSize: 1024 * 128,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	channels       map[uuid.UUID]*Channel
	createChannel  chan *Channel
	registerProxy  chan *Client
	registerTunnel chan *Client
	disconnected   chan *Client
}

func newHub() *Hub {
	return &Hub{
		channels:       make(map[uuid.UUID]*Channel),
		createChannel:  make(chan *Channel),
		registerProxy:  make(chan *Client),
		registerTunnel: make(chan *Client),
		disconnected:   make(chan *Client),
	}
}

type Channel struct {
	proxy   *Client //the proxy is on the network that we can't reach
	tunnel  *Client //the tunnel typically runs on our local computer
	id      uuid.UUID
	hub     *Hub
	handler func(*Channel)
}

type MessageWithWebsocketMessageType struct {
	message     []byte
	messageType int
}

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	otherSide  *Client
	channelId  uuid.UUID
	remoteType string
	params     map[string][]string
}

func (h *Hub) handleMessages() {
	log.Println("Waiting for messages on channels")
	for {
		select {
		case channel := <-h.createChannel:
			log.Printf("Creating new channel ID: %v", channel.id.String())
			h.channels[channel.id] = channel

		//the proxy is on the network that we can't reach
		case client := <-h.registerProxy:
			log.Printf("Registering proxy for channel ID: %v", client.channelId.String())
			if channel, ok := h.channels[client.channelId]; ok {
				channel.proxy = client
				if channel.tunnel != nil {
					log.Printf("Got both sides for channel ID: %v", client.channelId.String())
					channel.tunnel.otherSide = channel.proxy
					channel.proxy.otherSide = channel.tunnel
					log.Println("Launching channel handler")
					go channel.handler(channel)
				}
			} else {
				log.Printf("Registering proxy failed for channel ID %v, channel ID unknown\n", client.channelId.String())
			}

		//the tunnel typically runs on our local computer
		case client := <-h.registerTunnel:
			log.Printf("Registering tunnel for channel ID: %v", client.channelId.String())
			if channel, ok := h.channels[client.channelId]; ok {
				channel.tunnel = client
				if channel.proxy != nil {
					log.Printf("Got both sides for channel ID: %v", client.channelId.String())
					channel.tunnel.otherSide = channel.proxy
					channel.proxy.otherSide = channel.tunnel
					log.Println("Launching channel handler")
					go channel.handler(channel)
				}
			} else {
				log.Printf("Registering tunnel failed for channel ID %v, channel ID unknown\n", client.channelId.String())
			}

		//one of the sides disconnected, destroy the channel
		case client := <-h.disconnected:
			if channel, ok := h.channels[client.channelId]; ok {
				log.Printf("Destroying tunnel for channel ID: %v", channel.id.String())

				channel.proxy.otherSide = nil
				channel.tunnel.otherSide = nil

				channel.tunnel = nil
				channel.proxy = nil

				delete(h.channels, channel.id)
			}
		}
	}
}

func setRemote(hub *Hub, w http.ResponseWriter, r *http.Request, p httprouter.Params, register chan<- *Client, params map[string][]string) {
	id, _ := uuid.Parse(p.ByName("id"))

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer ws.Close()

	client := &Client{hub: hub, conn: ws, channelId: id, params: params}
	register <- client
	keepalive(ws)
}

func keepalive(ws *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	for {
		select {
		case <-ticker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func createChannel(hub *Hub, w http.ResponseWriter, r *http.Request, p httprouter.Params, channelHandler func(*Channel)) {
	log.Printf("Creating new channel")
	id := uuid.New()

	channel := &Channel{hub: hub, id: id, handler: channelHandler}
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

func main() {
	flag.Parse()
	hub := newHub()

	router := httprouter.New()
	router.NotFound = http.FileServer(http.Dir("public"))

	router.GET("/create", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		channelHandlerType := r.URL.Query().Get("type")
		var channelHandler func(*Channel)
		if len(channelHandlerType) == 0 || channelHandlerType == "tunnel" {
			channelHandlerType = "tunnel"
			channelHandler = Passthrough
		} else if channelHandlerType == "ssh" {
			channelHandler = sshShell
		}
		log.Println("Asked to create channel of type ", channelHandlerType)
		createChannel(hub, w, r, p, channelHandler)
	})

	router.GET("/ws/proxy/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		setRemote(hub, w, r, p, hub.registerProxy, r.URL.Query())
	})
	router.GET("/ws/tunnel/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		setRemote(hub, w, r, p, hub.registerTunnel, r.URL.Query())
	})

	log.Printf("Listening on %s\n", *addr)
	log.Printf("Pinging every %v seconds\n", pingPeriod)
	go hub.handleMessages()
	log.Fatal(http.ListenAndServe(*addr, router))
}
