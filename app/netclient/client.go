package netclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"device-agent/internal/security"
	"device-agent/internal/tcpserver"
)

type Config struct {
	ServerAddr string
	AppID      string
	SN         string
	Key        string
	Reconnect  ReconnectConfig
}

type ReconnectConfig struct {
	MinMS int
	MaxMS int
}

type Client struct {
	config     *Config
	conn       net.Conn
	auth       *security.Authenticator

	connected   bool
	connMu      sync.RWMutex
	writeMu     sync.Mutex

	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup

	onCommand   func(*tcpserver.CommandMessage)
	onConnected func(bool)

	lastError   error
}

func NewClient(config *Config, onCommand func(*tcpserver.CommandMessage)) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	keys := map[string]string{config.AppID: config.Key}
	auth := security.NewAuthenticator(keys, 300, security.NewMemoryNonceStore())

	return &Client{
		config:    config,
		auth:      auth,
		ctx:       ctx,
		cancel:    cancel,
		onCommand: onCommand,
	}
}

func (c *Client) SetConnectedCallback(callback func(bool)) {
	c.onConnected = callback
}

func (c *Client) Start() error {
	c.wg.Add(1)
	go c.reconnectLoop()
	return nil
}

func (c *Client) Stop() error {
	c.cancel()
	c.closeConnection()
	c.wg.Wait()
	return nil
}

func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

func (c *Client) GetLastError() error {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.lastError
}

func (c *Client) SendACK(cmdID, status, detail string) error {
	ack := &tcpserver.ACKMessage{
		CmdID:  cmdID,
		Status: status,
		Detail: detail,
	}

	msg, err := tcpserver.NewMessage(tcpserver.TypeACK, ack)
	if err != nil {
		return err
	}

	return c.sendMessage(msg)
}

func (c *Client) reconnectLoop() {
	defer c.wg.Done()

	backoff := time.Duration(c.config.Reconnect.MinMS) * time.Millisecond
	maxBackoff := time.Duration(c.config.Reconnect.MaxMS) * time.Millisecond

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if err := c.connect(); err != nil {
			c.setError(err)
			log.Printf("Connection failed: %v, retrying in %v", err, backoff)

			select {
			case <-c.ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = time.Duration(c.config.Reconnect.MinMS) * time.Millisecond
		c.setConnected(true)

		c.handleConnection()

		c.setConnected(false)
		c.closeConnection()
	}
}

func (c *Client) connect() error {
	conn, err := net.DialTimeout("tcp", c.config.ServerAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	return c.authenticate()
}

func (c *Client) authenticate() error {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	ts := time.Now().Unix()
	nonceStr := hex.EncodeToString(nonce)
	signature := c.auth.GenerateSignature(c.config.AppID, c.config.SN, ts, nonceStr, c.config.Key)

	auth := &tcpserver.AuthMessage{
		AppID: c.config.AppID,
		SN:    c.config.SN,
		TS:    ts,
		Nonce: nonceStr,
		Sign:  signature,
		Meta: map[string]string{
			"version": "1.0.0",
			"os":      "client",
		},
	}

	msg, err := tcpserver.NewMessage(tcpserver.TypeAuth, auth)
	if err != nil {
		return fmt.Errorf("create auth message: %w", err)
	}

	if err := c.sendMessage(msg); err != nil {
		return fmt.Errorf("send auth: %w", err)
	}

	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	response, err := tcpserver.ReadMessage(c.conn)
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}

	if response.Type != tcpserver.TypeAuthOK {
		return fmt.Errorf("unexpected auth response type: %d", response.Type)
	}

	var authOK tcpserver.AuthOKMessage
	if err := tcpserver.UnmarshalPayload(response.Payload, &authOK); err != nil {
		return fmt.Errorf("parse auth response: %w", err)
	}

	if !authOK.Success {
		return fmt.Errorf("auth failed: %s", authOK.Message)
	}

	log.Printf("Device %s authenticated successfully", c.config.SN)
	return nil
}

func (c *Client) handleConnection() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Connection handler panic: %v", r)
		}
	}()

	heartbeatTicker := time.NewTicker(25 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-heartbeatTicker.C:
			if err := c.sendPing(); err != nil {
				log.Printf("Send ping failed: %v", err)
				return
			}
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		msg, err := tcpserver.ReadMessage(c.conn)
		if err != nil {
			c.setError(err)
			return
		}

		if err := c.handleMessage(msg); err != nil {
			log.Printf("Handle message error: %v", err)
		}
	}
}

func (c *Client) handleMessage(msg *tcpserver.Message) error {
	switch msg.Type {
	case tcpserver.TypePing:
		return c.handlePing(msg)
	case tcpserver.TypeCMD:
		return c.handleCommand(msg)
	default:
		log.Printf("Unhandled message type: %d", msg.Type)
	}
	return nil
}

func (c *Client) handlePing(msg *tcpserver.Message) error {
	pong := &tcpserver.PongMessage{Timestamp: time.Now().Unix()}
	pongMsg, err := tcpserver.NewMessage(tcpserver.TypePong, pong)
	if err != nil {
		return err
	}
	return c.sendMessage(pongMsg)
}

func (c *Client) handleCommand(msg *tcpserver.Message) error {
	var cmd tcpserver.CommandMessage
	if err := tcpserver.UnmarshalPayload(msg.Payload, &cmd); err != nil {
		return fmt.Errorf("parse command: %w", err)
	}

	if c.onCommand != nil {
		go c.onCommand(&cmd)
	}

	return nil
}

func (c *Client) sendPing() error {
	ping := &tcpserver.PingMessage{Timestamp: time.Now().Unix()}
	msg, err := tcpserver.NewMessage(tcpserver.TypePing, ping)
	if err != nil {
		return err
	}
	return c.sendMessage(msg)
}

func (c *Client) sendMessage(msg *tcpserver.Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	return tcpserver.WriteMessage(conn, msg)
}

func (c *Client) closeConnection() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) setConnected(connected bool) {
	c.connMu.Lock()
	c.connected = connected
	if connected {
		c.lastError = nil
	}
	c.connMu.Unlock()

	if c.onConnected != nil {
		c.onConnected(connected)
	}
}

func (c *Client) setError(err error) {
	c.connMu.Lock()
	c.lastError = err
	c.connected = false
	c.connMu.Unlock()
}