package internal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"

	core "github.com/inoxlang/inox/internal/core"
	_dom "github.com/inoxlang/inox/internal/globals/dom"
)

const (
	DEFAULT_SESSION_ID_BYTE_COUNT = 16
	MAX_SESSION_ID_BYTE_COUNT     = 32
	DEFAULT_SESSION_ID_KEY        = "session-id"
)

var (
	sessions           = cmap.New[*Session]()
	MAX_SESSION_ID_LEN = hex.EncodedLen(MAX_SESSION_ID_BYTE_COUNT)

	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionIdTooLong = errors.New("session id is too long")
)

type Session struct {
	Id     string
	lock   sync.Mutex
	views  map[core.ResourceName]*_dom.View
	server *HttpServer
}

func (s *Session) GetView(ctx *core.Context, r core.ResourceName) (*_dom.View, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.views == nil {
		return nil, false
	}
	v, ok := s.views[r]
	return v, ok
}

func (s *Session) SetView(ctx *core.Context, r core.ResourceName, v *_dom.View) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.views == nil {
		s.views = map[core.ResourceName]*_dom.View{}
	}
	s.views[r] = v
}

func (s *Session) GetOrSetView(
	ctx *core.Context, r core.ResourceName, fn func() *_dom.View,
) (view *_dom.View, found bool, set bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.views == nil {
		s.views = map[core.ResourceName]*_dom.View{}
	}
	v, ok := s.views[r]
	if ok {
		return v, true, false
	}

	v = fn()
	if v == nil {
		return v, false, false
	}
	s.views[r] = v
	return v, false, true
}

func getSession(req *http.Request) (*Session, error) {
	for _, cookie := range req.Cookies() {
		if cookie.Name == DEFAULT_SESSION_ID_KEY {
			if len(cookie.Value) > MAX_SESSION_ID_LEN {
				return nil, ErrSessionIdTooLong
			}

			session, ok := sessions.Get(cookie.Value)
			if ok {
				return session, nil
			}
			return nil, ErrSessionNotFound
		}
	}

	return nil, ErrSessionNotFound
}

// addNewSession creates a new session an saves it in a global map.
func addNewSession(server *HttpServer) *Session {
	//random session ID
	var sessionId [DEFAULT_SESSION_ID_BYTE_COUNT]byte
	_, err := rand.Read(sessionId[:])

	if err != nil {
		panic(err)
	}

	sessionIdStr := hex.EncodeToString(sessionId[:])

	//create session & saves it in the sessions map
	session := &Session{
		Id:     sessionIdStr,
		server: server,
	}

	sessions.Set(sessionIdStr, session)
	return session
}

func addSessionIdCookie(rw *HttpResponseWriter, sessionId string) {
	http.SetCookie(rw.rw, &http.Cookie{
		Name:     DEFAULT_SESSION_ID_KEY,
		Value:    sessionId,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
	})
}
