package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
	"printnest/internal/db"
)

const CookieName = "printnest_session"

var ErrUnauthorized = errors.New("unauthorized")

func EnsurePassword(store *db.DB, seed string) error {
	var cur string
	err := store.SQL.QueryRow(`SELECT value FROM settings WHERE key='admin_password_hash'`).Scan(&cur)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if cur != "" {
		return nil
	}
	if seed == "" {
		return errors.New("no admin password configured: set PRINTNEST_PASSWORD on first boot")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(seed), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return store.SetSetting("admin_password_hash", string(hash))
}

func CheckPassword(store *db.DB, password string) bool {
	var hash string
	if err := store.SQL.QueryRow(`SELECT value FROM settings WHERE key='admin_password_hash'`).Scan(&hash); err != nil {
		return false
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return false
	}
	return true
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateSession(store *db.DB, ttl time.Duration) (token string, expires time.Time, err error) {
	token, err = newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now()
	expires = now.Add(ttl)
	_, err = store.SQL.Exec(`INSERT INTO sessions(id,created_at,expires_at) VALUES(?,?,?)`,
		token, now.Unix(), expires.Unix())
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func DestroySession(store *db.DB, token string) {
	store.SQL.Exec(`DELETE FROM sessions WHERE id=?`, token)
}

func ValidSession(store *db.DB, r *http.Request) bool {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return false
	}
	var exp int64
	if err := store.SQL.QueryRow(`SELECT expires_at FROM sessions WHERE id=?`, c.Value).Scan(&exp); err != nil {
		return false
	}
	if subtle.ConstantTimeCompare([]byte("1"), []byte("1")) != 1 {
		return false
	}
	if time.Now().Unix() > exp {
		store.SQL.Exec(`DELETE FROM sessions WHERE id=?`, c.Value)
		return false
	}
	return true
}

func RequireAuth(store *db.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ValidSession(store, r) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SetCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func SweepExpired(store *db.DB) {
	store.SQL.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
}
