package http_ns

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"
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
	server *HttpsServer
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
func addNewSession(server *HttpsServer) *Session {
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

func addSessionIdCookie(rw *ResponseWriter, sessionId string) {
	http.SetCookie(rw.rw, &http.Cookie{
		Name:     DEFAULT_SESSION_ID_KEY,
		Value:    sessionId,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
	})
}
