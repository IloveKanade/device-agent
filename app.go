package main

import (
	"context"
	"log"
	"os"

	"device-agent/app/controller"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// App struct
type App struct {
	ctx        context.Context
	controller *controller.Controller
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	config := a.loadConfig()
	a.controller = controller.NewController(config)

	// Set callbacks
	a.controller.SetOpenURLCallback(func(url string) {
		runtime.EventsEmit(ctx, "open-url", url)
	})

	a.controller.SetStatusCallback(func(status controller.Status) {
		runtime.EventsEmit(ctx, "status", status)
	})

	// Start the controller
	if err := a.controller.Start(ctx); err != nil {
		log.Printf("Failed to start controller: %v", err)
	}
}

func (a *App) loadConfig() *controller.Config {
	config := &controller.Config{}

	// Set defaults
	config.ServerAddr = "localhost:9001"
	config.AppID = "A1"
	config.SN = "SN123456"
	config.Key = "K_SECRET_ABC"
	config.OpenURL = "about:blank"
	config.Serve.Enable = true
	config.Serve.Addr = "127.0.0.1:18765"
	config.Serve.Root = "./static"
	config.Proxy.Enable = false
	config.Reconnect.MinMS = 500
	config.Reconnect.MaxMS = 15000

	// Try to load from file
	if data, err := os.ReadFile("configs/agent.yaml"); err == nil {
		if err := yaml.Unmarshal(data, config); err != nil {
			log.Printf("Failed to parse config file: %v", err)
		}
	}

	return config
}

// GetStatus returns the current connection status
func (a *App) GetStatus() controller.Status {
	if a.controller == nil {
		return controller.Status{Connected: false, LastErr: "not initialized"}
	}
	return a.controller.GetStatus()
}

// OpenURL opens a URL in the embedded browser
func (a *App) OpenURL(url string) {
	if a.controller != nil {
		a.controller.OpenURL(url)
	}
}
