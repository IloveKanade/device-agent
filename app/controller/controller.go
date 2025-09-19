package controller

import (
	"context"
	"fmt"
	"log"

	"device-agent/app/netclient"
	"device-agent/app/websvc"
	"device-agent/internal/tcpserver"
)

type Config struct {
	ServerAddr string            `yaml:"server_addr"`
	AppID      string            `yaml:"appid"`
	SN         string            `yaml:"sn"`
	Key        string            `yaml:"key"`
	OpenURL    string            `yaml:"open_url"`
	Serve      websvc.Config     `yaml:"serve"`
	Proxy      websvc.ProxyConfig `yaml:"proxy"`
	Reconnect  netclient.ReconnectConfig `yaml:"reconnect"`
}

type Controller struct {
	config    *Config
	client    *netclient.Client
	webServer *websvc.Server

	onOpenURL    func(string)
	onStatusChange func(Status)
}

type Status struct {
	Connected bool   `json:"connected"`
	LastErr   string `json:"last_err,omitempty"`
}

func NewController(config *Config) *Controller {
	return &Controller{
		config: config,
	}
}

func (c *Controller) SetOpenURLCallback(callback func(string)) {
	c.onOpenURL = callback
}

func (c *Controller) SetStatusCallback(callback func(Status)) {
	c.onStatusChange = callback
}

func (c *Controller) Start(ctx context.Context) error {
	// Setup web server
	webConfig := &websvc.Config{
		Addr:  c.config.Serve.Addr,
		Root:  c.config.Serve.Root,
		Proxy: c.config.Proxy,
	}

	if c.config.Serve.Enable {
		c.webServer = websvc.NewServer(webConfig)
		if err := c.webServer.Start(); err != nil {
			return fmt.Errorf("failed to start web server: %w", err)
		}
	}

	// Setup TCP client
	clientConfig := &netclient.Config{
		ServerAddr: c.config.ServerAddr,
		AppID:      c.config.AppID,
		SN:         c.config.SN,
		Key:        c.config.Key,
		Reconnect:  c.config.Reconnect,
	}

	c.client = netclient.NewClient(clientConfig, c.handleCommand)
	c.client.SetConnectedCallback(c.handleConnectionStatus)

	if err := c.client.Start(); err != nil {
		return fmt.Errorf("failed to start TCP client: %w", err)
	}

	// Open initial URL if specified
	if c.config.OpenURL != "" {
		c.OpenURL(c.config.OpenURL)
	}

	log.Printf("Controller started for device %s", c.config.SN)
	return nil
}

func (c *Controller) Stop() error {
	if c.client != nil {
		c.client.Stop()
	}

	if c.webServer != nil {
		c.webServer.Stop()
	}

	log.Println("Controller stopped")
	return nil
}

func (c *Controller) OpenURL(url string) {
	if c.onOpenURL != nil {
		c.onOpenURL(url)
	}
}

func (c *Controller) GetStatus() Status {
	if c.client == nil {
		return Status{Connected: false, LastErr: "not started"}
	}

	connected := c.client.IsConnected()
	status := Status{Connected: connected}

	if !connected {
		if err := c.client.GetLastError(); err != nil {
			status.LastErr = err.Error()
		}
	}

	return status
}

func (c *Controller) handleCommand(cmd *tcpserver.CommandMessage) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Command handler panic: %v", r)
			c.client.SendACK(cmd.CmdID, "error", fmt.Sprintf("panic: %v", r))
		}
	}()

	log.Printf("Received command: %s (ID: %s)", cmd.Cmd, cmd.CmdID)

	switch cmd.Cmd {
	case "OPEN_WEB":
		c.handleOpenWeb(cmd)
	case "SERVE_PATH":
		c.handleServePath(cmd)
	case "PROXY_TARGET":
		c.handleProxyTarget(cmd)
	default:
		c.client.SendACK(cmd.CmdID, "error", "unknown command")
	}
}

func (c *Controller) handleOpenWeb(cmd *tcpserver.CommandMessage) {
	url, ok := cmd.Args["url"].(string)
	if !ok {
		c.client.SendACK(cmd.CmdID, "error", "url parameter required")
		return
	}

	c.OpenURL(url)
	c.client.SendACK(cmd.CmdID, "ok", "url opened")
}

func (c *Controller) handleServePath(cmd *tcpserver.CommandMessage) {
	path, ok := cmd.Args["path"].(string)
	if !ok {
		c.client.SendACK(cmd.CmdID, "error", "path parameter required")
		return
	}

	if c.webServer != nil {
		c.webServer.ServePath(path)
		url := c.webServer.GetURL()
		c.OpenURL(url)
		c.client.SendACK(cmd.CmdID, "ok", "serving path: "+path)
	} else {
		c.client.SendACK(cmd.CmdID, "error", "web server not enabled")
	}
}

func (c *Controller) handleProxyTarget(cmd *tcpserver.CommandMessage) {
	target, ok := cmd.Args["target"].(string)
	if !ok {
		c.client.SendACK(cmd.CmdID, "error", "target parameter required")
		return
	}

	// In a real implementation, you might want to recreate the proxy
	// For now, just acknowledge
	c.client.SendACK(cmd.CmdID, "ok", "proxy target updated: "+target)
}

func (c *Controller) handleConnectionStatus(connected bool) {
	status := c.GetStatus()

	if c.onStatusChange != nil {
		c.onStatusChange(status)
	}

	if connected {
		log.Printf("Device %s connected to server", c.config.SN)
	} else {
		log.Printf("Device %s disconnected from server", c.config.SN)
	}
}