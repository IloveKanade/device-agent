package api

import (
	"time"

	"device-agent/internal/tcpserver"

	"github.com/gin-gonic/gin"
)

func SetupSimpleRouter(sessionManager *tcpserver.SessionManager, ackWaiter *tcpserver.ACKWaiter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery(), RequestID(), AccessLog(), CORS())

	deviceCtl := NewDeviceController(sessionManager)
	msgCtl := NewMessageController(sessionManager, ackWaiter)

	api := r.Group("/api")
	{
		devices := api.Group("/devices")
		{
			devices.GET("/online", deviceCtl.ListOnline)
			devices.GET("/offline", deviceCtl.ListOffline)
			devices.GET("/:sn", deviceCtl.GetDetail)
			devices.POST("/:sn/send", msgCtl.SendToDevice)
			devices.POST("/:sn/send-async", msgCtl.SendAsync)
		}
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"time":   time.Now().Unix(),
		})
	})

	return r
}