package main

import (
	"context"
	"log"
	"time"

	"device-agent/app/netclient"
	"device-agent/internal/tcpserver"
)

func main() {
	config := &netclient.Config{
		ServerAddr: "localhost:9001",
		AppID:      "A1",
		SN:         "SN123456",
		Key:        "K_SECRET_ABC",
		Reconnect: netclient.ReconnectConfig{
			MinMS: 500,
			MaxMS: 15000,
		},
	}

	client := netclient.NewClient(config, handleCommand)
	globalClient = client
	client.SetConnectedCallback(func(connected bool) {
		if connected {
			log.Printf("✅ Device %s connected to server", config.SN)
		} else {
			log.Printf("❌ Device %s disconnected from server", config.SN)
			if err := client.GetLastError(); err != nil {
				log.Printf("Last error: %v", err)
			}
		}
	})

	log.Printf("Starting test client for device %s", config.SN)
	if err := client.Start(); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}

	// Keep running
	ctx := context.Background()
	<-ctx.Done()
}

var globalClient *netclient.Client

func handleCommand(cmd *tcpserver.CommandMessage) {
	log.Printf("📨 Received command: %s (ID: %s)", cmd.Cmd, cmd.CmdID)
	log.Printf("Arguments: %+v", cmd.Args)

	// Simulate command processing
	switch cmd.Cmd {
	case "OPEN_WEB":
		url, ok := cmd.Args["url"].(string)
		if !ok {
			log.Printf("❌ Invalid URL in command")
			globalClient.SendACK(cmd.CmdID, "error", "invalid url parameter")
			return
		}
		log.Printf("🌐 Would open URL: %s", url)
		time.Sleep(500 * time.Millisecond) // Simulate work
		log.Printf("✅ Command %s completed successfully", cmd.CmdID)
		globalClient.SendACK(cmd.CmdID, "ok", "url opened: "+url)

	case "SERVE_PATH":
		path, ok := cmd.Args["path"].(string)
		if !ok {
			log.Printf("❌ Invalid path in command")
			globalClient.SendACK(cmd.CmdID, "error", "invalid path parameter")
			return
		}
		log.Printf("📁 Would serve path: %s", path)
		time.Sleep(500 * time.Millisecond) // Simulate work
		log.Printf("✅ Command %s completed successfully", cmd.CmdID)
		globalClient.SendACK(cmd.CmdID, "ok", "serving path: "+path)

	default:
		log.Printf("❓ Unknown command: %s", cmd.Cmd)
		globalClient.SendACK(cmd.CmdID, "error", "unknown command")
	}
}