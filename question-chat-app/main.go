package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// Question represents a Q&A message
type Question struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "waiting", "answered"
	Category  string    `json:"category"`
}

// Client represents a connected WebSocket client
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Username string
	Send     chan []byte
	IsAdmin  bool
}

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
	redis      *redis.Client
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func newHub() *Hub {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       1, // Different DB for question app
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Redis bağlantısı kurulamadı: %v", err)
		log.Println("Redis olmadan devam ediliyor...")
		rdb = nil
	} else {
		log.Println("Redis bağlantısı başarılı")
	}

	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		redis:      rdb,
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("Yeni kullanıcı bağlandı. ID: %s", client.ID)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
				if client.Username != "" {
					log.Printf("Kullanıcı ayrıldı: %s", client.Username)
				}
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	indexPath := filepath.Join(".", "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		log.Printf("index.html dosyası bulunamadı: %s", indexPath)
		http.Error(w, "index.html file not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, indexPath)
}

func serveWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade hatası: %v", err)
		return
	}

	clientID := fmt.Sprintf("%d%.3f", time.Now().Unix(), time.Now().Sub(time.Unix(time.Now().Unix(), 0)).Seconds())
	client := &Client{
		ID:   clientID,
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump(hub)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var question Question
		if err := json.Unmarshal(messageBytes, &question); err != nil {
			log.Printf("Mesaj parse hatası: %v", err)
			continue
		}

		question.Timestamp = time.Now()
		question.ID = fmt.Sprintf("%d", time.Now().UnixNano())

		if question.Username != "" {
			c.Username = question.Username
		}

		enrichedMessage, err := json.Marshal(question)
		if err != nil {
			log.Printf("Mesaj JSON encode hatası: %v", err)
			continue
		}

		hub.broadcast <- enrichedMessage
	}
}

func main() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWS(hub, w, r)
	})

	// Container içinde HTTP modunda çalış (Nginx SSL termination yapar)
	log.Printf("HTTP Question Chat sunucusu :80 portunda başlatıldı...")
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal("HTTP ListenAndServe hatası: ", err)
	}
}
