package booster

import (
	"sync/atomic"
)

type Hub struct {
	broadcast  chan *envelope
	register   chan *Session
	unRegister chan *Session

	closed uint32
	exit   chan bool
	exited chan bool

	sessions map[string][]*Session
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan *envelope),
		register:   make(chan *Session),
		unRegister: make(chan *Session),
		exit:       make(chan bool),
		exited:     make(chan bool),

		sessions: make(map[string][]*Session),
	}
}

func (h *Hub) Run() {
LOOP:
	for {
		select {
		case <-h.exit:
			atomic.StoreUint32(&h.closed, 1)

			for key, ss := range h.sessions {
				for _, s := range ss {
					s.Close()
					close(s.output)
					<-s.exited
				}
				delete(h.sessions, key)
			}

			break LOOP

		case s := <-h.register:
			if h.Closed() {
				break
			}

			if _, ok := h.sessions[s.userId]; !ok {
				h.sessions[s.userId] = make([]*Session, 0)
			}

			h.sessions[s.userId] = append(h.sessions[s.userId], s)

		case s := <-h.unRegister:
			if h.Closed() {
				break
			}

			if ss, ok := h.sessions[s.userId]; ok {
				for i, s1 := range ss {
					if s1 == s {
						h.sessions[s.userId] = append(h.sessions[s.userId][:i], h.sessions[s.userId][i+1:]...)
						break
					}
				}
			}

		case e := <-h.broadcast:
			if h.Closed() {
				break
			}

			if len(e.userIds) > 0 {
				for _, userId := range e.userIds {
					ss, ok := h.sessions[userId]
					if !ok {
						continue
					}
					for _, s := range ss {
						if e.filter != nil && !e.filter(s) {
							continue
						}
						s.output <- e
					}
				}
				break
			}

			for _, ss := range h.sessions {
				for _, s := range ss {
					if e.filter != nil && !e.filter(s) {
						continue
					}
					s.output <- e
				}
			}

		}
	}

	h.exited <- true
}

func (h *Hub) Close() {
	close(h.exit)
	<-h.exited
}

func (h *Hub) Closed() bool {
	if atomic.LoadUint32(&h.closed) == 1 {
		return true
	}

	return false
}
