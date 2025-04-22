package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
)

const (
	sessionFile = "json/sessions.json"

	AdminSessionConst = "admin_session"
	UserSessionConst  = "user_session"

	SessionDuration = 12 * time.Hour

	AdminRole = "admin"
	UserRole  = "user"
)

var (
	sessionMu sync.Mutex
)

type Session struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func GenerateToken() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func loadSessions() ([]Session, error) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return []Session{}, nil
	}

	var sessions []Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

func saveSessions(sessions []Session) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionFile, data, 0644)
}

func SetSession(sess Session) error {

	type sessionValueMap struct {
		Index int
		Session
	}

	sessions, err := loadSessions()
	if err != nil {
		return err
	}

	sessionMap := map[string]sessionValueMap{}

	for i, s := range sessions {
		sessionMap[s.Username] = sessionValueMap{
			Index:   i,
			Session: s,
		}
	}

	s, exists := sessionMap[sess.Username]
	if exists {
		sessions[s.Index] = sess
	} else {
		sessions = append(sessions, sess)
	}

	return saveSessions(sessions)
}

func ValidateSession(role, token string) bool {
	sessions, err := loadSessions()
	if err != nil {
		return false
	}

	validSessions := []Session{}
	isValid := false

	for _, session := range sessions {
		if session.Token == token && session.Role == role {
			if time.Now().Before(session.ExpiresAt) {
				isValid = true
				validSessions = append(validSessions, session)
			}
		} else {
			validSessions = append(validSessions, session)
		}
	}

	_ = saveSessions(validSessions)

	return isValid
}

func CleanupSessions() {
	for {
		sessions, err := loadSessions()
		if err != nil {
			panic(err)
		}

		validSessions := []Session{}

		for _, s := range sessions {
			if !time.Now().After(s.ExpiresAt) {
				validSessions = append(validSessions, s)
			}
		}

		_ = saveSessions(validSessions)

		time.Sleep(1 * time.Minute)
	}
}
