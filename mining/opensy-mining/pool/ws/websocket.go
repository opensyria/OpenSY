package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"net/http"

	"github.com/gorilla/websocket"
)

// Config holds WebSocket server configuration
type Config struct {
	Logger         *slog.Logger
	StatsInterval  time.Duration
	PingInterval   time.Duration
	WriteTimeout   time.Duration
	ReadTimeout    time.Duration
	MaxMessageSize int64
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		Logger:         slog.Default(),
		StatsInterval:  2 * time.Second,
		PingInterval:   30 * time.Second,
		WriteTimeout:   10 * time.Second,
		ReadTimeout:    60 * time.Second,
		MaxMessageSize: 4096,
	}
}

// MessageType represents WebSocket message types
type MessageType string

const (
	MsgTypeStats       MessageType = "stats"
	MsgTypeNewBlock    MessageType = "new_block"
	MsgTypeNewShare    MessageType = "new_share"
	MsgTypeSubscribe   MessageType = "subscribe"
	MsgTypeUnsubscribe MessageType = "unsubscribe"
	MsgTypePing        MessageType = "ping"
	MsgTypePong        MessageType = "pong"
	MsgTypeError       MessageType = "error"
)

// Message is a WebSocket message
type Message struct {
	Type      MessageType     `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// StatsData holds pool statistics
type StatsData struct {
	Hashrate    float64 `json:"hashrate"`
	Miners      int64   `json:"miners"`
	Workers     int64   `json:"workers"`
	BlocksFound int64   `json:"blocks_found"`
	Difficulty  float64 `json:"difficulty"`
	Height      int64   `json:"height"`
}

// BlockData holds new block information
type BlockData struct {
	Height    int64  `json:"height"`
	Hash      string `json:"hash"`
	Reward    int64  `json:"reward"`
	MinerAddr string `json:"miner_address"`
	Timestamp int64  `json:"timestamp"`
}

// ShareData holds share submission information
type ShareData struct {
	MinerAddr  string `json:"miner_address"`
	WorkerName string `json:"worker_name"`
	Difficulty uint64 `json:"difficulty"`
	Valid      bool   `json:"valid"`
}

// SubscribeRequest is a subscription request
type SubscribeRequest struct {
	Channels []string `json:"channels"` // "stats", "blocks", "shares", "miner:<address>"
}

// Client represents a WebSocket client
type Client struct {
	ID            string
	conn          *websocket.Conn
	server        *Server
	send          chan []byte
	subscriptions map[string]bool
	mu            sync.RWMutex
	minerAddr     string // If authenticated/subscribed to specific miner
}

// Server is the WebSocket server
type Server struct {
	upgrader websocket.Upgrader
	clients  map[string]*Client
	mu       sync.RWMutex
	logger   *slog.Logger

	// Channels for broadcasting
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client

	// Data providers
	statsProvider func() *StatsData

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new WebSocket server
func NewServer(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Configure properly for production
			},
		},
		clients:    make(map[string]*Client),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     cfg.Logger.With("component", "websocket"),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetStatsProvider sets the stats data provider
func (s *Server) SetStatsProvider(fn func() *StatsData) {
	s.statsProvider = fn
}

// Start starts the WebSocket server
func (s *Server) Start() {
	s.wg.Add(1)
	go s.run()
	s.wg.Add(1)
	go s.statsBroadcaster()
}

// Stop stops the WebSocket server
func (s *Server) Stop() {
	s.cancel()
	// Close all client connections
	s.mu.Lock()
	for _, client := range s.clients {
		client.conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// Handler returns the HTTP handler for WebSocket connections
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			s.logger.Error("Failed to upgrade connection", "error", err)
			return
		}

		client := &Client{
			ID:            generateClientID(),
			conn:          conn,
			server:        s,
			send:          make(chan []byte, 256),
			subscriptions: make(map[string]bool),
		}

		s.register <- client

		go client.writePump()
		go client.readPump()
	}
}

func (s *Server) run() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client.ID] = client
			s.mu.Unlock()
			s.logger.Debug("Client connected", "id", client.ID)
		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client.ID]; ok {
				delete(s.clients, client.ID)
				close(client.send)
			}
			s.mu.Unlock()
			s.logger.Debug("Client disconnected", "id", client.ID)
		case msg := <-s.broadcast:
			data, _ := json.Marshal(msg)
			s.mu.RLock()
			for _, client := range s.clients {
				if client.shouldReceive(msg) {
					select {
					case client.send <- data:
					default:
						// Client buffer full, skip
					}
				}
			}
			s.mu.RUnlock()
		}
	}
}

func (s *Server) statsBroadcaster() {
	defer s.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if s.statsProvider == nil {
				continue
			}
			stats := s.statsProvider()
			if stats == nil {
				continue
			}
			data, _ := json.Marshal(stats)
			s.broadcast <- &Message{
				Type:      MsgTypeStats,
				Data:      data,
				Timestamp: time.Now().Unix(),
			}
		}
	}
}

// BroadcastBlock broadcasts a new block to all clients
func (s *Server) BroadcastBlock(block *BlockData) {
	data, _ := json.Marshal(block)
	s.broadcast <- &Message{
		Type:      MsgTypeNewBlock,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

// BroadcastShare broadcasts a new share to relevant clients
func (s *Server) BroadcastShare(share *ShareData) {
	data, _ := json.Marshal(share)
	s.broadcast <- &Message{
		Type:      MsgTypeNewShare,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

// Client methods

func (c *Client) shouldReceive(msg *Message) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Always receive stats
	if msg.Type == MsgTypeStats && c.subscriptions["stats"] {
		return true
	}

	// Block notifications
	if msg.Type == MsgTypeNewBlock && c.subscriptions["blocks"] {
		return true
	}

	// Share notifications
	if msg.Type == MsgTypeNewShare && c.subscriptions["shares"] {
		return true
	}

	return false
}

func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.server.logger.Debug("WebSocket error", "error", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		c.handleMessage(&msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case MsgTypeSubscribe:
		var req SubscribeRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}
		c.mu.Lock()
		for _, ch := range req.Channels {
			c.subscriptions[ch] = true
		}
		c.mu.Unlock()

	case MsgTypeUnsubscribe:
		var req SubscribeRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}
		c.mu.Lock()
		for _, ch := range req.Channels {
			delete(c.subscriptions, ch)
		}
		c.mu.Unlock()

	case MsgTypePing:
		response := Message{
			Type:      MsgTypePong,
			Timestamp: time.Now().Unix(),
		}
		data, _ := json.Marshal(response)
		select {
		case c.send <- data:
		default:
		}
	}
}

func generateClientID() string {
	// In production, use crypto/rand
	return time.Now().Format("20060102150405.000000")
}
