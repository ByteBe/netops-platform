// Package notify WebSocket 实时推送中心（链路实时监控、监控数据、大屏刷新）
package notify

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message 推送消息
type Message struct {
	Channel string `json:"channel"` // linkdetect/monitor/dashboard/traffic/dbmonitor
	Type    string `json:"type"`    // 事件类型
	Data    any    `json:"data"`
	TS      int64  `json:"ts"`
}

// Client WebSocket 客户端
type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	sendOnce sync.Once // 保证 send 通道只关闭一次（Unsubscribe 与 Close 可能并发触发）
	channels map[string]bool
	user     string
	mu       sync.Mutex
	closed   bool
}

// Hub 推送中心
type Hub struct {
	mu       sync.RWMutex
	clients  map[*Client]bool
	register chan *Client
	unreg    chan *Client
	stopped  bool
}

// NewHub 创建推送中心
func NewHub() *Hub {
	h := &Hub{
		clients:  make(map[*Client]bool),
		register: make(chan *Client),
		unreg:    make(chan *Client),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
		case c := <-h.unreg:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				c.sendOnce.Do(func() { close(c.send) })
			}
			h.mu.Unlock()
		}
	}
}

// Subscribe 订阅客户端
func (h *Hub) Subscribe(c *Client) { h.register <- c }

// Unsubscribe 退订
func (h *Hub) Unsubscribe(c *Client) { h.unreg <- c }

// AddClient 构造客户端
func (h *Hub) AddClient(conn *websocket.Conn, channels []string, user string) *Client {
	chs := map[string]bool{}
	for _, ch := range channels {
		chs[ch] = true
	}
	if len(chs) == 0 {
		chs["*"] = true
	}
	return &Client{conn: conn, send: make(chan []byte, 64), channels: chs, user: user}
}

// Publish 向指定频道推送（无客户端订阅 * 时忽略）
func (h *Hub) Publish(channel string, msgType string, data any) {
	h.mu.RLock()
	payload, err := json.Marshal(Message{Channel: channel, Type: msgType, Data: data, TS: time.Now().UnixMilli()})
	if err != nil {
		h.mu.RUnlock()
		return
	}
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		if c.channels["*"] || c.channels[channel] {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range clients {
		select {
		case c.send <- payload:
		default:
			// 慢消费者丢弃
		}
	}
}

// SendLoop 客户端发送循环（外部调用）
func (c *Client) SendLoop() {
	for payload := range c.send {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		err := c.conn.WriteMessage(websocket.TextMessage, payload)
		c.mu.Unlock()
		if err != nil {
			return
		}
	}
}

// Close 关闭客户端
func (c *Client) Close() {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		_ = c.conn.Close()
	}
	c.mu.Unlock()
	c.sendOnce.Do(func() { close(c.send) })
}

// Stop 停止中心
func (h *Hub) Stop() {
	h.mu.Lock()
	h.stopped = true
	for c := range h.clients {
		c.Close()
	}
	h.mu.Unlock()
}
