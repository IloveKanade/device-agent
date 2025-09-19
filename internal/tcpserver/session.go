package tcpserver

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID         string
	SN         string
	AppID      string
	Conn       net.Conn
	RemoteAddr string
	LoginAt    time.Time
	LastPing   time.Time
	Meta       map[string]string

	writeMu    sync.Mutex
	closeCh    chan struct{}
	closeOnce  sync.Once
}

func NewSession(conn net.Conn) *Session {
	return &Session{
		ID:         uuid.New().String(),
		Conn:       conn,
		RemoteAddr: conn.RemoteAddr().String(),
		LoginAt:    time.Now(),
		LastPing:   time.Now(),
		closeCh:    make(chan struct{}),
	}
}

func (s *Session) SendMessage(msg *Message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	select {
	case <-s.closeCh:
		return ErrSessionClosed
	default:
		return WriteMessage(s.Conn, msg)
	}
}

func (s *Session) SendCommand(cmd *CommandMessage) error {
	msg, err := NewMessage(TypeCMD, cmd)
	if err != nil {
		return err
	}
	return s.SendMessage(msg)
}

func (s *Session) SendPing() error {
	ping := &PingMessage{Timestamp: time.Now().Unix()}
	msg, err := NewMessage(TypePing, ping)
	if err != nil {
		return err
	}
	return s.SendMessage(msg)
}

func (s *Session) UpdatePing() {
	s.LastPing = time.Now()
}

func (s *Session) IsExpired(timeout time.Duration) bool {
	return time.Since(s.LastPing) > timeout
}

func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
		s.Conn.Close()
	})
	return nil
}

func (s *Session) IsClosed() bool {
	select {
	case <-s.closeCh:
		return true
	default:
		return false
	}
}

type SessionManager struct {
	sessions map[string]*Session
	snToID   map[string]string
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		snToID:   make(map[string]string),
	}
}

func (sm *SessionManager) Add(session *Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session.SN != "" {
		if oldID, exists := sm.snToID[session.SN]; exists {
			if oldSession, ok := sm.sessions[oldID]; ok {
				oldSession.Close()
				delete(sm.sessions, oldID)
			}
		}
		sm.snToID[session.SN] = session.ID
	}

	sm.sessions[session.ID] = session
}

func (sm *SessionManager) Remove(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.Close()
		delete(sm.sessions, sessionID)

		if session.SN != "" {
			delete(sm.snToID, session.SN)
		}
	}
}

func (sm *SessionManager) Get(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, exists := sm.sessions[sessionID]
	return session, exists
}

func (sm *SessionManager) GetBySN(sn string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sessionID, exists := sm.snToID[sn]; exists {
		if session, ok := sm.sessions[sessionID]; ok && !session.IsClosed() {
			return session, true
		}
	}
	return nil, false
}

func (sm *SessionManager) GetOnlineDevices() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var devices []string
	for sn := range sm.snToID {
		devices = append(devices, sn)
	}
	return devices
}

func (sm *SessionManager) GetSessionInfo() map[string]SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	info := make(map[string]SessionInfo)
	for _, session := range sm.sessions {
		info[session.ID] = SessionInfo{
			ID:         session.ID,
			SN:         session.SN,
			AppID:      session.AppID,
			RemoteAddr: session.RemoteAddr,
			LoginAt:    session.LoginAt,
			LastPing:   session.LastPing,
			Meta:       session.Meta,
		}
	}
	return info
}

func (sm *SessionManager) CleanupExpired(timeout time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var expired []string
	for id, session := range sm.sessions {
		if session.IsExpired(timeout) {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		if session, exists := sm.sessions[id]; exists {
			session.Close()
			delete(sm.sessions, id)
			if session.SN != "" {
				delete(sm.snToID, session.SN)
			}
		}
	}

	return len(expired)
}

func (sm *SessionManager) Shutdown(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, session := range sm.sessions {
		session.Close()
	}

	sm.sessions = make(map[string]*Session)
	sm.snToID = make(map[string]string)
	return nil
}

type SessionInfo struct {
	ID         string            `json:"id"`
	SN         string            `json:"sn"`
	AppID      string            `json:"appid"`
	RemoteAddr string            `json:"remote_addr"`
	LoginAt    time.Time         `json:"login_at"`
	LastPing   time.Time         `json:"last_ping"`
	Meta       map[string]string `json:"meta"`
}

var ErrSessionClosed = fmt.Errorf("session closed")