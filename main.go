package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/csrf"
)

type ViewData struct {
	Username       string
	LocalStatuses  []ServiceStatus
	RemoteStatuses map[string][]ServiceStatus
	LocalCommands  []Command
	RemoteCommands map[string][]Command
}

func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)
	AppLogger.Printf("MANUAL REFRESH: user=%s IP=%s", session.Username, r.RemoteAddr)

	refreshLocalStatus()
	refreshRemoteStatus()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)
	user := GetUser(session.Username)

	viewData := ViewData{
		Username:       user.Username,
		LocalStatuses:  []ServiceStatus{},
		RemoteStatuses: make(map[string][]ServiceStatus),
		LocalCommands:  []Command{},
		RemoteCommands: make(map[string][]Command),
	}

	// Load statuses (safe concurrent read)
	StatusData.RLock()
	viewData.LocalStatuses = append(viewData.LocalStatuses, StatusData.Local...)
	for k, v := range StatusData.Remote {
		viewData.RemoteStatuses[k] = append([]ServiceStatus{}, v...)
	}
	StatusData.RUnlock()

	// Filter commands based on user permissions
	for _, c := range AppConfig.Commands {
		if IsCommandAllowed(user, c.Name) {
			if c.Type == "local" {
				viewData.LocalCommands = append(viewData.LocalCommands, c)
			} else if c.Type == "remote" {
				viewData.RemoteCommands[c.RemoteName] = append(viewData.RemoteCommands[c.RemoteName], c)
			}
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, viewData)
}

var csrfKey []byte = generateRandomCSRFKey()
var csrfMiddleware func(http.Handler) http.Handler

func main() {
	InitLogger()
	LoadJail()
	SaveJail()

	err := LoadConfig("config.json")
	if err != nil {
		AppLogger.Fatalf("Failed to load config: %v", err)
	}

	useHTTPS := AppConfig.TLSCert != "" && AppConfig.TLSKey != ""

	if useHTTPS {
		csrfOptions := []csrf.Option{
			csrf.SameSite(csrf.SameSiteLaxMode),
			csrf.Secure(true),
		}
		csrfMiddleware = csrf.Protect(csrfKey, csrfOptions...)
		AppLogger.Printf("Using HTTPS — CSRF protection is enabled")
	} else {
		AppLogger.Printf("Using HTTP — CSRF protection is disabled")
	}

	http.Handle("/", ApplyMiddlewares(http.HandlerFunc(IndexHandler), true, true))
	http.Handle("/login", ApplyMiddlewares(http.HandlerFunc(LoginHandler), true, false))
	http.Handle("/logout", ApplyMiddlewares(http.HandlerFunc(LogoutHandler), false, false))
	http.Handle("/run", ApplyMiddlewares(http.HandlerFunc(RunHandler), false, true))
	http.Handle("/status", ApplyMiddlewares(http.HandlerFunc(StatusAPIHandler), false, true))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	address := fmt.Sprintf("%s:%s", AppConfig.ServerIP, AppConfig.ServerPort)
	AppLogger.Printf("Starting server on %s", address)

	if useHTTPS {
		AppLogger.Printf("Using HTTPS cert %s key %s", AppConfig.TLSCert, AppConfig.TLSKey)
		err = http.ListenAndServeTLS(address, AppConfig.TLSCert, AppConfig.TLSKey, nil)
	} else {
		AppLogger.Printf("Using HTTP")
		err = http.ListenAndServe(address, nil)
	}

	if err != nil {
		AppLogger.Fatalf("Server failed: %v", err)
	}
}
