package tcpserver

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

const (
	ProtocolVersion = 1
	MaxMessageSize  = 1024 * 1024 // 1MB
)

type MessageType uint8

const (
	TypeAuth    MessageType = 1
	TypeAuthOK  MessageType = 2
	TypePing    MessageType = 3
	TypePong    MessageType = 4
	TypeReport  MessageType = 5
	TypeCMD     MessageType = 6
	TypeACK     MessageType = 7
	TypeErr     MessageType = 8
)

type Message struct {
	Version uint8       `json:"version"`
	Type    MessageType `json:"type"`
	Payload []byte      `json:"payload"`
}

type AuthMessage struct {
	AppID string            `json:"appid"`
	SN    string            `json:"sn"`
	TS    int64             `json:"ts"`
	Nonce string            `json:"nonce"`
	Sign  string            `json:"sign"`
	Meta  map[string]string `json:"meta"`
}

type AuthOKMessage struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type PingMessage struct {
	Timestamp int64 `json:"timestamp"`
}

type PongMessage struct {
	Timestamp int64 `json:"timestamp"`
}

type CommandMessage struct {
	CmdID     string                 `json:"cmd_id"`
	Cmd       string                 `json:"cmd"`
	Args      map[string]interface{} `json:"args"`
	TimeoutMS int                    `json:"timeout_ms"`
}

type ACKMessage struct {
	CmdID  string `json:"cmd_id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type ErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func ReadMessage(conn net.Conn) (*Message, error) {
	var lengthBuf [4]byte
	if _, err := io.ReadFull(conn, lengthBuf[:]); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	msgBuf := make([]byte, length)
	if _, err := io.ReadFull(conn, msgBuf); err != nil {
		return nil, fmt.Errorf("read message: %w", err)
	}

	if length < 2 {
		return nil, fmt.Errorf("message too short")
	}

	msg := &Message{
		Version: msgBuf[0],
		Type:    MessageType(msgBuf[1]),
		Payload: msgBuf[2:],
	}

	if msg.Version != ProtocolVersion {
		return nil, fmt.Errorf("unsupported protocol version: %d", msg.Version)
	}

	return msg, nil
}

func WriteMessage(conn net.Conn, msg *Message) error {
	msgBuf := make([]byte, 2+len(msg.Payload))
	msgBuf[0] = msg.Version
	msgBuf[1] = uint8(msg.Type)
	copy(msgBuf[2:], msg.Payload)

	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(msgBuf)))

	fullBuf := append(lengthBuf, msgBuf...)
	_, err := conn.Write(fullBuf)
	return err
}

func MarshalPayload(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func UnmarshalPayload(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	data, err := MarshalPayload(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Version: ProtocolVersion,
		Type:    msgType,
		Payload: data,
	}, nil
}