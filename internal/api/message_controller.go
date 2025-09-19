package api

import (
	"encoding/json"
	"net/http"
	"time"

	"device-agent/internal/tcpserver"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MessageController struct {
	sessionManager *tcpserver.SessionManager
	ackWaiter      *tcpserver.ACKWaiter
}

func NewMessageController(sessionManager *tcpserver.SessionManager, ackWaiter *tcpserver.ACKWaiter) *MessageController {
	return &MessageController{
		sessionManager: sessionManager,
		ackWaiter:      ackWaiter,
	}
}

type SendMessageRequest struct {
	MsgType   string          `json:"msg_type" binding:"required"`
	Payload   json.RawMessage `json:"payload" binding:"required"`
	TimeoutMS int             `json:"timeout_ms"`
}

type SendMessageResponse struct {
	Success bool                    `json:"success"`
	CmdID   string                  `json:"cmd_id,omitempty"`
	ACK     *tcpserver.ACKMessage   `json:"ack,omitempty"`
	Error   string                  `json:"error,omitempty"`
}

func (mc *MessageController) SendToDevice(c *gin.Context) {
	sn := c.Param("sn")
	if sn == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "sn parameter is required",
		})
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request: " + err.Error(),
		})
		return
	}

	session, exists := mc.sessionManager.GetBySN(sn)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "device offline",
		})
		return
	}

	cmdID := uuid.New().String()

	var args map[string]interface{}
	if err := json.Unmarshal(req.Payload, &args); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid payload: " + err.Error(),
		})
		return
	}

	cmd := &tcpserver.CommandMessage{
		CmdID:     cmdID,
		Cmd:       req.MsgType,
		Args:      args,
		TimeoutMS: req.TimeoutMS,
	}

	if err := session.SendCommand(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to send command: " + err.Error(),
		})
		return
	}

	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ack, err := mc.ackWaiter.Wait(cmdID, timeout)
	if err != nil {
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"success": false,
			"error":   "ack timeout: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"cmd_id":  cmdID,
		"ack":     ack,
	})
}

func (mc *MessageController) SendAsync(c *gin.Context) {
	sn := c.Param("sn")
	if sn == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "sn parameter is required",
		})
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request: " + err.Error(),
		})
		return
	}

	session, exists := mc.sessionManager.GetBySN(sn)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "device offline",
		})
		return
	}

	cmdID := uuid.New().String()

	var args map[string]interface{}
	if err := json.Unmarshal(req.Payload, &args); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid payload: " + err.Error(),
		})
		return
	}

	cmd := &tcpserver.CommandMessage{
		CmdID:     cmdID,
		Cmd:       req.MsgType,
		Args:      args,
		TimeoutMS: req.TimeoutMS,
	}

	if err := session.SendCommand(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to send command: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"cmd_id":  cmdID,
		"message": "command sent",
	})
}