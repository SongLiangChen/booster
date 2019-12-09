package booster

import (
	"fmt"
	"github.com/gorilla/websocket"
	"strconv"
	"sync"
	"time"
)

// A Session structure represents a remote peer
type Session struct {
	conn    *websocket.Conn
	output  chan *envelope
	booster *Booster

	userId string
	appId  string

	// keys save the customized business data
	keys map[string]interface{}

	// params save input data from http request and SHOULD read only
	params map[string]string

	exited chan bool

	sync.RWMutex
}

// Get session's userId
func (s *Session) GetUserId() string {
	return s.userId
}

// Get session's appId
func (s *Session) GetAppId() string {
	return s.appId
}

// Put message to WritePump
func (s *Session) writeMessage(message *envelope) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	select {
	case s.output <- message:
	default:
		s.booster.errorHandler(s, fmt.Errorf("write channel full, abandon message[%v]", message))
	}
}

// Send message to peer immediately
func (s *Session) writeRaw(message *envelope) error {
	s.conn.SetWriteDeadline(time.Now().Add(s.booster.config.WriteWait))
	err := s.conn.WriteMessage(message.t, message.msg)

	if err != nil {
		return err
	}

	if message.t == websocket.CloseMessage {
		err := s.conn.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// Send a close message to peer immediately
func (s *Session) close() {
	s.writeRaw(&envelope{t: websocket.CloseMessage, msg: []byte{}})
}

// Send a ping message to peer immediately
func (s *Session) ping() {
	s.writeRaw(&envelope{t: websocket.PingMessage, msg: []byte{}})
}

// WritePump async send message to remote peer
func (s *Session) writePump() {
	defer s.conn.Close()

	ticker := time.NewTicker(s.booster.config.PingPeriod)
	defer ticker.Stop()

loop:
	for {
		select {
		case msg, ok := <-s.output:
			if !ok {
				s.close()
				break loop
			}

			if err := s.writeRaw(msg); err != nil {
				s.booster.errorHandler(s, err)
				break loop
			}

		case <-ticker.C:
			s.ping()
		}
	}

	s.exited <- true
}

// ReadPump receive message from remote peer, and turn message to messageHandler
func (s *Session) readPump() {
	defer s.conn.Close()

	s.conn.SetReadLimit(s.booster.config.MaxMessageSize)

	s.conn.SetPongHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(s.booster.config.PongWait))
		return nil
	})

	for {
		s.conn.SetReadDeadline(time.Now().Add(s.booster.config.PongWait))

		t, message, err := s.conn.ReadMessage()

		if err != nil {
			s.booster.errorHandler(s, err)
			break
		}

		if t == websocket.CloseMessage {
			break
		}

		s.booster.messageHandler(s, message)
	}
}

// Write message to session.
func (s *Session) Write(msg []byte) {
	s.writeMessage(&envelope{t: websocket.TextMessage, msg: msg})
}

// Write binary message to session.
func (s *Session) WriteBinary(msg []byte) {
	s.writeMessage(&envelope{t: websocket.BinaryMessage, msg: msg})
}

// Close session.
func (s *Session) Close() {
	s.writeMessage(&envelope{t: websocket.CloseMessage, msg: []byte{}})
}

// ---------------------------------------------------
//             customized business data
// ---------------------------------------------------

// Store a key-val pair in session
func (s *Session) Set(key string, val interface{}) {
	s.Lock()
	s.keys[key] = val
	s.Unlock()
}

// Get val from session by key
func (s *Session) Get(key string) interface{} {
	s.RLock()
	defer s.RUnlock()

	return s.keys[key]
}

// Must get val from session by key, panic if not found
func (s *Session) MustGet(key string) interface{} {
	val := s.Get(key)
	if val == nil {
		panic("Session MustGet:" + key + " fail")
	}
	return val
}

// Get a string val from session by key
func (s *Session) GetString(key string) string {
	val := s.Get(key)
	if val != nil {
		if v, ok := val.(string); ok {
			return v
		}
	}

	return ""
}

// Get a int val from session by key
func (s *Session) GetInt(key string) int {
	val := s.Get(key)
	if val != nil {
		if v, ok := val.(int); ok {
			return v
		}
	}

	return 0
}

// Get a int64 val from session by key
func (s *Session) GetInt64(key string) int64 {
	val := s.Get(key)
	if val != nil {
		if v, ok := val.(int64); ok {
			return v
		}
	}

	return 0
}

// ---------------------------------------------------
//               http params
// ---------------------------------------------------

// GetParam returns a string from params
func (s *Session) GetParam(key string) string {
	return s.params[key]
}

// GetParamInt returns a int from params, returns 0 if key not exist
func (s *Session) GetParamInt(key string) int {
	val := s.GetParam(key)
	if val == "" {
		return 0
	}

	n, _ := strconv.Atoi(val)
	return n
}

// GetParamInt64 returns a int64 from params, returns 0 if key not exist
func (s *Session) GetParamInt64(key string) int64 {
	val := s.GetParam(key)
	if val == "" {
		return 0
	}

	n, _ := strconv.ParseInt(val, 10, 64)
	return n
}
