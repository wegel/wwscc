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

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (writeWait * 5) / 10
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
	registerClient chan *Client
	disconnected   chan *Client
}

func newHub() *Hub {
	return &Hub{
		channels:       make(map[uuid.UUID]*Channel),
		createChannel:  make(chan *Channel),
		registerClient: make(chan *Client),
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

func (h *Hub) setClient(client *Client) {
	if channel, ok := h.channels[client.channelID]; ok {
		if client.remoteType == "tunnel" {
			channel.tunnel = client
		} else if client.remoteType == "proxy" {
			channel.proxy = client
		}

		if channel.tunnel != nil && channel.proxy != nil {
			log.Printf("Got both sides for channel ID: %v", client.channelID.String())
			channel.tunnel.otherSide = channel.proxy
			channel.proxy.otherSide = channel.tunnel
			log.Println("Launching channel handler")
			go channel.handler(channel)
		}
	} else {
		log.Printf("Registering proxy failed for channel ID %v, channel ID unknown\n", client.channelID.String())

		go func(client *Client) {
			//tar trap potential attacker
			time.Sleep(30 * time.Second)
			client.ws.Close()
		}(client)
	}
}

func (h *Hub) handleMessages() {
	log.Println("Waiting for messages on channels")
	for {
		select {
		case channel := <-h.createChannel:
			log.Printf("Creating new channel ID: %v", channel.id.String())
			h.channels[channel.id] = channel

		//the proxy is on the network that we can't reach
		case client := <-h.registerClient:
			log.Printf("Registering %s for channel ID: %v", client.remoteType, client.channelID.String())
			h.setClient(client)

		//one of the sides disconnected, destroy the channel
		case client := <-h.disconnected:
			if channel, ok := h.channels[client.channelID]; ok {
				log.Printf("Destroying tunnel for channel ID: %v", channel.id.String())
				if channel.proxy != nil {
					if channel.proxy.ws != nil {
						channel.proxy.ws.Close()
					}
					channel.proxy.otherSide = nil
					channel.proxy = nil
				}
				if channel.tunnel != nil {
					if channel.tunnel.ws != nil {
						channel.tunnel.ws.Close()
					}
					channel.tunnel.otherSide = nil
					channel.tunnel = nil
				}
				delete(h.channels, channel.id)
			}
		}
	}
}

func setRemote(hub *Hub, w http.ResponseWriter, r *http.Request, channelID uuid.UUID, remoteType string, params map[string][]string) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer ws.Close()

	client := &Client{hub: hub, ws: ws, channelID: channelID, params: params, remoteType: remoteType}
	hub.registerClient <- client
	keepalive(client)
}

func keepalive(client *Client) {
	defer func() {
		client.hub.disconnected <- client
		if r := recover(); r != nil {
			log.Printf("error in keepalive for %s on %s: %v", client.remoteType, client.channelID, r)
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	for {
		select {
		case <-ticker.C:
			if err := client.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				panic(err)
			}
			if err := client.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				panic(err)
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
		log.Println("Asked to create channel of type", channelHandlerType)
		createChannel(hub, w, r, p, channelHandler)
	})

	router.GET("/ws/proxy/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		id, _ := uuid.Parse(p.ByName("id"))
		setRemote(hub, w, r, id, "proxy", r.URL.Query())
	})
	router.GET("/ws/tunnel/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		id, _ := uuid.Parse(p.ByName("id"))
		setRemote(hub, w, r, id, "tunnel", r.URL.Query())
	})

	log.Printf("Listening on %s\n", *addr)
	log.Printf("Pinging every %v seconds\n", pingPeriod)
	go hub.handleMessages()
	log.Fatal(http.ListenAndServe(*addr, router))
}
