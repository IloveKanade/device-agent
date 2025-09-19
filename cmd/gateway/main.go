package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"device-agent/internal/api"
	"device-agent/internal/tcpserver"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TCP struct {
		Addr              string        `yaml:"addr"`
		HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
		SessionTimeout    time.Duration `yaml:"session_timeout"`
		TimeWindowSec     int64         `yaml:"time_window_sec"`
	} `yaml:"tcp"`
	HTTP struct {
		Addr string `yaml:"addr"`
	} `yaml:"http"`
	Auth struct {
		Keys map[string]string `yaml:"keys"`
	} `yaml:"auth"`
}

func main() {
	config := loadConfig()

	tcpConfig := &tcpserver.Config{
		Addr:              config.TCP.Addr,
		HeartbeatInterval: config.TCP.HeartbeatInterval,
		SessionTimeout:    config.TCP.SessionTimeout,
		Keys:              config.Auth.Keys,
		TimeWindowSec:     config.TCP.TimeWindowSec,
	}

	tcpServer := tcpserver.NewServer(tcpConfig)
	if err := tcpServer.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}

	router := api.SetupSimpleRouter(tcpServer.GetSessionManager(), tcpServer.GetACKWaiter())
	httpServer := &http.Server{
		Addr:    config.HTTP.Addr,
		Handler: router,
	}

	go func() {
		log.Printf("HTTP server listening on %s", config.HTTP.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server forced to shutdown: %v", err)
	}

	if err := tcpServer.Stop(); err != nil {
		log.Printf("TCP server forced to shutdown: %v", err)
	}

	log.Println("Servers stopped")
}

func loadConfig() *Config {
	config := &Config{}

	// Set defaults
	config.TCP.Addr = ":9001"
	config.TCP.HeartbeatInterval = 30 * time.Second
	config.TCP.SessionTimeout = 90 * time.Second
	config.TCP.TimeWindowSec = 300 // 5 minutes
	config.HTTP.Addr = ":8080"
	config.Auth.Keys = map[string]string{
		"A1": "K_SECRET_ABC",
	}

	// Try to load from file
	if data, err := os.ReadFile("configs/gateway.yaml"); err == nil {
		if err := yaml.Unmarshal(data, config); err != nil {
			log.Printf("Failed to parse config file: %v", err)
		}
	}

	return config
}