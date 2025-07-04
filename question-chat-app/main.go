package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// Message represents a chat message
type Message struct {
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Channel   string    `json:"channel"`
	Type      string    `json:"type,omitempty"` // "text", "file", "image"
	FileURL   string    `json:"fileUrl,omitempty"`
	FileName  string    `json:"fileName,omitempty"`
	FileSize  int64     `json:"fileSize,omitempty"`
}

// Client represents a connected WebSocket client
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Username string
	Send     chan []byte
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
		return true // Allow connections from any origin
	},
}

func newHub() *Hub {
	// Redis client configuration
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       1, // Use different DB for question chat
	})

	// Test Redis connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Redis bağlantısı kurulamadı: %v", err)
		log.Println("Redis olmadan devam ediliyor...")
		rdb = nil
	} else {
		log.Println("Question Chat Redis bağlantısı başarılı")
	}

	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		redis:      rdb,
	}
}

// Store message in Redis
func (h *Hub) storeMessage(msg Message) {
	if h.redis == nil {
		return
	}

	ctx := context.Background()
	messageJSON, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Mesaj serialize hatası: %v", err)
		return
	}

	key := fmt.Sprintf("question_messages:%s", msg.Channel)
	pipe := h.redis.Pipeline()
	pipe.LPush(ctx, key, messageJSON)
	pipe.LTrim(ctx, key, 0, 99)
	pipe.Expire(ctx, key, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Printf("Redis mesaj kaydetme hatası: %v", err)
	}
}

// Get recent messages from Redis
func (h *Hub) getRecentMessages(channel string, limit int) ([]Message, error) {
	if h.redis == nil {
		return []Message{}, nil
	}

	ctx := context.Background()
	key := fmt.Sprintf("question_messages:%s", channel)

	results, err := h.redis.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(results))
	for i := len(results) - 1; i >= 0; i-- {
		var msg Message
		if err := json.Unmarshal([]byte(results[i]), &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

func (h *Hub) sendRecentMessages(client *Client, channel string) {
	messages, err := h.getRecentMessages(channel, 50)
	if err != nil {
		log.Printf("Geçmiş mesajları alma hatası: %v", err)
		return
	}

	for _, msg := range messages {
		messageJSON, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		select {
		case client.Send <- messageJSON:
		default:
			log.Printf("İstemci gönderim buffer'ı dolu, mesaj atlandı")
		}
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("Question Chat: Yeni kullanıcı bağlandı. ID: %s", client.ID)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
				if client.Username != "" {
					log.Printf("Question Chat: Kullanıcı ayrıldı: %s", client.Username)
				}
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			var msg Message
			if err := json.Unmarshal(message, &msg); err == nil {
				h.storeMessage(msg)
			}

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

			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket hatası: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("Mesaj parse hatası: %v", err)
			msg = Message{
				Username:  "Anonim",
				Message:   string(messageBytes),
				Timestamp: time.Now(),
				Channel:   "soru-cevap",
			}
		} else {
			if msg.Username != "" {
				c.Username = msg.Username
			}
			msg.Timestamp = time.Now()
			if msg.Channel == "" {
				msg.Channel = "soru-cevap"
			}

			if msg.Message == "__GET_RECENT_MESSAGES__" {
				go hub.sendRecentMessages(c, msg.Channel)
				continue
			}
		}

		enrichedMessage, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Mesaj JSON encode hatası: %v", err)
			continue
		}

		hub.broadcast <- enrichedMessage
	}
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

func handleFileUpload(hub *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Printf("Dosya parse hatası: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Dosya alma hatası: %v", err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	username := r.FormValue("username")
	channel := r.FormValue("channel")

	if username == "" || channel == "" {
		http.Error(w, "Missing username or channel", http.StatusBadRequest)
		return
	}

	if header.Size > 10*1024*1024 {
		log.Printf("Dosya çok büyük: %d bytes", header.Size)
		http.Error(w, "File size too large (max 10MB)", http.StatusBadRequest)
		return
	}

	// File type validation
	allowedTypes := map[string]bool{
		"image/jpeg": true, "image/png": true, "image/gif": true,
		"image/webp": true, "image/bmp": true, "text/plain": true,
		"application/pdf": true, "application/zip": true,
		"application/x-zip-compressed": true, "application/rar": true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".pdf":
			contentType = "application/pdf"
		case ".txt":
			contentType = "text/plain"
		case ".zip":
			contentType = "application/zip"
		case ".doc":
			contentType = "application/msword"
		case ".docx":
			contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		default:
			http.Error(w, "Unsupported file type", http.StatusBadRequest)
			return
		}
	}

	if !allowedTypes[contentType] {
		log.Printf("İzin verilmeyen dosya tipi: %s", contentType)
		http.Error(w, "File type not allowed", http.StatusBadRequest)
		return
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	ext := filepath.Ext(header.Filename)
	baseName := strings.TrimSuffix(header.Filename, ext)
	baseName = strings.ReplaceAll(baseName, " ", "_")
	baseName = strings.ReplaceAll(baseName, "..", "")
	baseName = strings.ReplaceAll(baseName, "/", "_")
	baseName = strings.ReplaceAll(baseName, "\\", "_")

	fileName := fmt.Sprintf("%d_%s%s", timestamp, baseName, ext)

	// Create uploads directory
	uploadsDir := "../uploads"
	dateDir := time.Now().Format("2006-01-02")
	fullUploadDir := filepath.Join(uploadsDir, "question-chat", dateDir)

	if err := os.MkdirAll(fullUploadDir, 0755); err != nil {
		log.Printf("Upload klasörü oluşturma hatası: %v", err)
		http.Error(w, "Error creating uploads directory", http.StatusInternalServerError)
		return
	}

	// Save file
	filePath := filepath.Join(fullUploadDir, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("Dosya oluşturma hatası: %v", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		log.Printf("Dosya kopyalama hatası: %v", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	log.Printf("Question Chat dosya kaydedildi: %s (%d bytes)", filePath, written)

	// Determine message type
	messageType := "file"
	if strings.HasPrefix(contentType, "image/") {
		messageType = "image"
	}

	// Create file message
	fileURL := fmt.Sprintf("/uploads/question-chat/%s/%s", dateDir, fileName)
	fileMessage := Message{
		Username:  username,
		Message:   fmt.Sprintf("Dosya paylaştı: %s", header.Filename),
		Timestamp: time.Now(),
		Channel:   channel,
		Type:      messageType,
		FileURL:   fileURL,
		FileName:  header.Filename,
		FileSize:  header.Size,
	}

	// Broadcast file message
	messageJSON, err := json.Marshal(fileMessage)
	if err != nil {
		log.Printf("Dosya mesajı marshalling hatası: %v", err)
		http.Error(w, "Error processing file message", http.StatusInternalServerError)
		return
	}

	hub.broadcast <- messageJSON

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "File uploaded successfully",
		"fileUrl":  fileURL,
		"fileName": header.Filename,
		"fileSize": header.Size,
	})
}

func main() {
	hub := newHub()
	go hub.run()

	// Create uploads directory
	uploadsDir := "../uploads/question-chat"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Printf("Question Chat uploads klasörü oluşturulamadı: %v", err)
	}

	// Static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("../uploads/"))))

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWS(hub, w, r)
	})
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		handleFileUpload(hub, w, r)
	})

	log.Printf("Question Chat HTTP sunucusu :80 portunda başlatıldı...")
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal("HTTP ListenAndServe hatası: ", err)
	}
}
