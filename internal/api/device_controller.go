package api

import (
	"net/http"

	"device-agent/internal/tcpserver"

	"github.com/gin-gonic/gin"
)

type DeviceController struct {
	sessionManager *tcpserver.SessionManager
}

func NewDeviceController(sessionManager *tcpserver.SessionManager) *DeviceController {
	return &DeviceController{
		sessionManager: sessionManager,
	}
}

type DeviceInfo struct {
	SN         string            `json:"sn"`
	AppID      string            `json:"appid"`
	RemoteAddr string            `json:"remote_addr"`
	LoginAt    string            `json:"login_at"`
	LastPing   string            `json:"last_ping"`
	Meta       map[string]string `json:"meta"`
	Online     bool              `json:"online"`
}

func (dc *DeviceController) ListOnline(c *gin.Context) {
	devices := dc.sessionManager.GetOnlineDevices()

	var result []DeviceInfo
	for _, sn := range devices {
		if session, exists := dc.sessionManager.GetBySN(sn); exists {
			result = append(result, DeviceInfo{
				SN:         session.SN,
				AppID:      session.AppID,
				RemoteAddr: session.RemoteAddr,
				LoginAt:    session.LoginAt.Format("2006-01-02 15:04:05"),
				LastPing:   session.LastPing.Format("2006-01-02 15:04:05"),
				Meta:       session.Meta,
				Online:     true,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"count":   len(result),
	})
}

func (dc *DeviceController) ListOffline(c *gin.Context) {
	// In a real implementation, this would query a database for offline devices
	// For now, return empty list as we only track online sessions in memory
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    []DeviceInfo{},
		"count":   0,
	})
}

func (dc *DeviceController) GetDetail(c *gin.Context) {
	sn := c.Param("sn")
	if sn == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "sn parameter is required",
		})
		return
	}

	session, exists := dc.sessionManager.GetBySN(sn)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "device not found",
		})
		return
	}

	device := DeviceInfo{
		SN:         session.SN,
		AppID:      session.AppID,
		RemoteAddr: session.RemoteAddr,
		LoginAt:    session.LoginAt.Format("2006-01-02 15:04:05"),
		LastPing:   session.LastPing.Format("2006-01-02 15:04:05"),
		Meta:       session.Meta,
		Online:     true,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
	})
}