package main

import (
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/csrf"
)

const SessionCookieName = "sysnotadmin_session"
const SessionTimeout = 30 * time.Minute

var Sessions = make(map[string]Session)

type Session struct {
	Username  string
	ExpiresAt time.Time
}

func AuthenticateUser(username, password string) bool {
	for _, u := range AppConfig.Users {
		if u.Username == username && u.Password == password {
			return true
		}
	}
	return false
}

func GetUser(username string) *User {
	for _, u := range AppConfig.Users {
		if u.Username == username {
			return &u
		}
	}
	return nil
}

func SetSession(w http.ResponseWriter, username string) {
	sessionID := GenerateSessionID()
	Sessions[sessionID] = Session{
		Username:  username,
		ExpiresAt: time.Now().Add(SessionTimeout),
	}
	cookie := http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  Sessions[sessionID].ExpiresAt,
		HttpOnly: true,
	}
	http.SetCookie(w, &cookie)
}

func GetSession(r *http.Request) *Session {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil
	}
	session, ok := Sessions[cookie.Value]
	if !ok || time.Now().After(session.ExpiresAt) {
		return nil
	}
	return &session
}

func RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r)
		if session == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// fallback: just strip port manually
		parts := strings.Split(remoteAddr, ":")
		if len(parts) > 0 {
			return parts[0]
		}
		return remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}

	// If IPv4-mapped IPv6, convert to plain IPv4
	if ip.To4() != nil {
		return ip.To4().String()
	}
	return ip.String()
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	ip := parseIP(r.RemoteAddr)

	if IsIPBanned(ip) {
		AppLogger.Printf("[JAIL] BLOCKED attempt from banned IP: %s", ip)
		http.NotFound(w, r)
		return
	}

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if AuthenticateUser(username, password) {
			AppLogger.Printf("LOGIN SUCCESS: user=%s IP=%s", username, ip)
			SetSession(w, username)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		} else {
			AppLogger.Printf("LOGIN FAIL: user=%s IP=%s", username, ip)
			RecordFailedLogin(ip)
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/login.html"))
	tmpl.Execute(w, map[string]interface{}{
		"csrfField": func() template.HTML {
			if csrfMiddleware != nil {
				AppLogger.Printf("Origin: %s | Referer: %s", r.Header.Get("Origin"), r.Header.Get("Referer"))
				return csrf.TemplateField(r)
			}
			return ""
		}(),
	})
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/login", http.StatusFound)
}
