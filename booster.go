package booster

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

type handleMessageFunc func(*Session, []byte)
type handleErrorFunc func(*Session, error)
type handleCloseFunc func(*Session, int, string) error
type handleSessionFunc func(*Session)
type FilterFunc func(*Session) bool

const (
	CloseNormalClosure           = 1000
	CloseGoingAway               = 1001
	CloseProtocolError           = 1002
	CloseUnsupportedData         = 1003
	CloseNoStatusReceived        = 1005
	CloseAbnormalClosure         = 1006
	CloseInvalidFramePayloadData = 1007
	ClosePolicyViolation         = 1008
	CloseMessageTooBig           = 1009
	CloseMandatoryExtension      = 1010
	CloseInternalServerErr       = 1011
	CloseServiceRestart          = 1012
	CloseTryAgainLater           = 1013
	CloseTLSHandshake            = 1015
)

var MessageOfCode = map[int]string{
	CloseNormalClosure:           "CloseNormalClosure",
	CloseGoingAway:               "CloseGoingAway",
	CloseProtocolError:           "CloseProtocolError",
	CloseUnsupportedData:         "CloseUnsupportedData",
	CloseNoStatusReceived:        "CloseNoStatusReceived",
	CloseAbnormalClosure:         "CloseAbnormalClosure",
	CloseInvalidFramePayloadData: "CloseInvalidFramePayloadData",
	ClosePolicyViolation:         "ClosePolicyViolation",
	CloseMessageTooBig:           "CloseMessageTooBig",
	CloseMandatoryExtension:      "CloseMandatoryExtension",
	CloseInternalServerErr:       "CloseInternalServerErr",
	CloseServiceRestart:          "CloseServiceRestart",
	CloseTryAgainLater:           "CloseTryAgainLater",
	CloseTLSHandshake:            "CloseTLSHandshake",
}

type Booster struct {
	config            *Config
	upGrader          *websocket.Upgrader
	messageHandler    handleMessageFunc
	errorHandler      handleErrorFunc
	closeHandler      handleCloseFunc
	connectHandler    handleSessionFunc
	disconnectHandler handleSessionFunc
	hubs              map[string]*Hub

	sync.RWMutex
}

func NewBooster() *Booster {
	upGrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	return &Booster{
		config:   newConfig(),
		upGrader: upGrader,
		hubs:     make(map[string]*Hub),
	}
}

// HandleMessage called when data is received from peer.
func (b *Booster) HandleMessage(fn func(*Session, []byte)) {
	b.messageHandler = fn
}

// HandleError called when any error happen
func (b *Booster) HandleError(fn func(*Session, error)) {
	b.errorHandler = fn
}

// HandleClose called when peer close the conn
func (b *Booster) HandleClose(fn func(*Session, int, string) error) {
	b.closeHandler = fn
}

// HandleConnect called when peer connect
func (b *Booster) HandleConnect(fn func(*Session)) {
	b.connectHandler = fn
}

// HandleDisConnect called when peer disconnect
func (b *Booster) HandleDisConnect(fn func(*Session)) {
	b.disconnectHandler = fn
}

// PushMessage push the msg to user.
// fn is a filter function, and will called before msg send,
// if fn return false, the msg will not send
func (b *Booster) PushMessage(appId string, userIds []string, msg []byte, fn func(*Session) bool) error {
	h := b.getHub(appId)
	if h.Closed() {
		return fmt.Errorf("appId[%v] hub has closed", appId)
	}

	message := &envelope{t: websocket.TextMessage, userIds: userIds, msg: msg, filter: fn}
	h.broadcast <- message
	return nil
}

// HandleWs is the access interface, it can be register to gin's router for use
//
// for example:
// engine := gin.New()
// engine.Get("/ws", booster.GetInstance().HandleWs)
//
func (b *Booster) HandleWs(c *gin.Context) {
	var (
		h      = NewHelper(c)
		appId  = h.StringParam("appId")
		userId = h.StringParam("userId")
	)

	if appId == "" || userId == "" {
		log.Errorf("[HandleWs] appId[%v], userId[%v] [invalid params]", appId, userId)
		return
	}

	hub := b.getHub(appId)
	if hub.Closed() {
		log.Error("[HandleWs] appId[%v], userId[%v] [hub already closed]", appId, userId)
		return
	}

	conn, err := b.upGrader.Upgrade(c.Writer, c.Request, c.Writer.Header())
	if err != nil {
		log.Error("[HandleWs] appId[%v], userId[%v] [upgrade fail, %v]", appId, userId, err.Error())
		return
	}

	session := &Session{
		conn:    conn,
		output:  make(chan *envelope, b.config.MessageBufferSize),
		booster: b,
		appId:   appId,
		userId:  userId,
		keys:    make(map[string]interface{}),
		params:  h.GetParams(),
		exited:  make(chan bool),
	}

	if b.connectHandler != nil {
		b.connectHandler(session)
	}

	hub.register <- session

	go session.writePump()
	session.readPump()

	hub.unRegister <- session

	if b.disconnectHandler != nil {
		b.disconnectHandler(session)
	}
}

// CloseBooster release booster instance
func (b *Booster) CloseBooster() {
	for _, hub := range b.hubs {
		hub.Close()
	}
}

func (b *Booster) getHub(appId string) *Hub {
	b.RLock()
	if _, ok := b.hubs[appId]; !ok {
		b.RUnlock()

		h := NewHub()
		go h.Run()

		b.Lock()
		b.hubs[appId] = h
		b.Unlock()

		return h
	}

	h := b.hubs[appId]
	b.RUnlock()
	return h
}

// ---------------------------------------------------------------------------------------------------------------------

var booster *Booster

func init() {
	booster = NewBooster()
}

func GetInstance() *Booster {
	return booster
}
