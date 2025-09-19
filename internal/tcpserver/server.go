package tcpserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"device-agent/internal/security"
)

type Server struct {
	addr           string
	listener       net.Listener
	sessionManager *SessionManager
	authenticator  *security.Authenticator
	ackWaiter      *ACKWaiter

	handlers       map[MessageType]MessageHandler

	heartbeatInterval time.Duration
	sessionTimeout    time.Duration

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdown   chan struct{}
}

type MessageHandler func(*Session, *Message) error

type Config struct {
	Addr              string
	HeartbeatInterval time.Duration
	SessionTimeout    time.Duration
	Keys              map[string]string
	TimeWindowSec     int64
}

func NewServer(config *Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	nonceStore := security.NewMemoryNonceStore()
	authenticator := security.NewAuthenticator(config.Keys, config.TimeWindowSec, nonceStore)

	s := &Server{
		addr:              config.Addr,
		sessionManager:    NewSessionManager(),
		authenticator:     authenticator,
		ackWaiter:         NewACKWaiter(),
		handlers:          make(map[MessageType]MessageHandler),
		heartbeatInterval: config.HeartbeatInterval,
		sessionTimeout:    config.SessionTimeout,
		ctx:               ctx,
		cancel:            cancel,
		shutdown:          make(chan struct{}),
	}

	s.registerDefaultHandlers()
	return s
}

func (s *Server) registerDefaultHandlers() {
	s.handlers[TypeAuth] = s.handleAuth
	s.handlers[TypePing] = s.handlePing
	s.handlers[TypePong] = s.handlePong
	s.handlers[TypeACK] = s.handleACK
}

func (s *Server) RegisterHandler(msgType MessageType, handler MessageHandler) {
	s.handlers[msgType] = handler
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.listener = listener
	log.Printf("TCP server listening on %s", s.addr)

	s.wg.Add(3)
	go s.acceptLoop()
	go s.heartbeatLoop()
	go s.cleanupLoop()

	return nil
}

func (s *Server) Stop() error {
	s.cancel()
	close(s.shutdown)

	if s.listener != nil {
		s.listener.Close()
	}

	s.sessionManager.Shutdown(s.ctx)
	s.wg.Wait()

	log.Println("TCP server stopped")
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	session := NewSession(conn)
	defer session.Close()

	log.Printf("New connection from %s (session: %s)", session.RemoteAddr, session.ID)

	for {
		select {
		case <-s.shutdown:
			return
		case <-session.closeCh:
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(s.sessionTimeout))
		msg, err := ReadMessage(conn)
		if err != nil {
			log.Printf("Session %s read error: %v", session.ID, err)
			return
		}

		if err := s.handleMessage(session, msg); err != nil {
			log.Printf("Session %s handle error: %v", session.ID, err)
			s.sendError(session, 500, err.Error())
		}
	}
}

func (s *Server) handleMessage(session *Session, msg *Message) error {
	handler, exists := s.handlers[msg.Type]
	if !exists {
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}

	return handler(session, msg)
}

func (s *Server) handleAuth(session *Session, msg *Message) error {
	var auth AuthMessage
	if err := UnmarshalPayload(msg.Payload, &auth); err != nil {
		return fmt.Errorf("invalid auth payload: %w", err)
	}

	if err := s.authenticator.VerifySignature(auth.AppID, auth.SN, auth.TS, auth.Nonce, auth.Sign); err != nil {
		s.sendAuthResult(session, false, err.Error())
		return fmt.Errorf("auth failed: %w", err)
	}

	session.SN = auth.SN
	session.AppID = auth.AppID
	session.Meta = auth.Meta

	s.sessionManager.Add(session)
	s.sendAuthResult(session, true, "authenticated")

	log.Printf("Device %s authenticated (session: %s)", auth.SN, session.ID)
	return nil
}

func (s *Server) handlePing(session *Session, msg *Message) error {
	var ping PingMessage
	if err := UnmarshalPayload(msg.Payload, &ping); err != nil {
		return err
	}

	session.UpdatePing()

	pong := &PongMessage{Timestamp: time.Now().Unix()}
	pongMsg, err := NewMessage(TypePong, pong)
	if err != nil {
		return err
	}

	return session.SendMessage(pongMsg)
}

func (s *Server) handlePong(session *Session, msg *Message) error {
	session.UpdatePing()
	return nil
}

func (s *Server) handleACK(session *Session, msg *Message) error {
	var ack ACKMessage
	if err := UnmarshalPayload(msg.Payload, &ack); err != nil {
		return err
	}

	s.ackWaiter.Notify(ack.CmdID, &ack)
	return nil
}

func (s *Server) sendAuthResult(session *Session, success bool, message string) {
	authOK := &AuthOKMessage{
		Success: success,
		Message: message,
	}

	msg, err := NewMessage(TypeAuthOK, authOK)
	if err != nil {
		log.Printf("Failed to create auth result message: %v", err)
		return
	}

	if err := session.SendMessage(msg); err != nil {
		log.Printf("Failed to send auth result: %v", err)
	}
}

func (s *Server) sendError(session *Session, code int, message string) {
	errMsg := &ErrorMessage{
		Code:    code,
		Message: message,
	}

	msg, err := NewMessage(TypeErr, errMsg)
	if err != nil {
		log.Printf("Failed to create error message: %v", err)
		return
	}

	if err := session.SendMessage(msg); err != nil {
		log.Printf("Failed to send error: %v", err)
	}
}

func (s *Server) heartbeatLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			sessions := s.sessionManager.GetSessionInfo()
			for _, info := range sessions {
				if session, exists := s.sessionManager.Get(info.ID); exists && session.SN != "" {
					if err := session.SendPing(); err != nil {
						log.Printf("Failed to send ping to %s: %v", session.SN, err)
						s.sessionManager.Remove(session.ID)
					}
				}
			}
		}
	}
}

func (s *Server) cleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			expired := s.sessionManager.CleanupExpired(s.sessionTimeout)
			if expired > 0 {
				log.Printf("Cleaned up %d expired sessions", expired)
			}
		}
	}
}

func (s *Server) GetSessionManager() *SessionManager {
	return s.sessionManager
}

func (s *Server) GetACKWaiter() *ACKWaiter {
	return s.ackWaiter
}