package main

import (
	"fmt"
	"html/template"
	"net/http"
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

func main() {
	InitLogger()
	LoadJail()
	SaveJail()

	err := LoadConfig("config.json")
	if err != nil {
		AppLogger.Fatalf("Failed to load config: %v", err)
	}

	go StatusRefresher()

	http.HandleFunc("/", RequireLogin(IndexHandler))
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/run", RequireLogin(RunHandler))
	http.HandleFunc("/refresh", RequireLogin(RefreshHandler))
	http.HandleFunc("/status", RequireLogin(StatusAPIHandler))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	address := fmt.Sprintf("%s:%s", AppConfig.ServerIP, AppConfig.ServerPort)
	AppLogger.Printf("Starting server on %s", address)

	if AppConfig.TLSCert != "" && AppConfig.TLSKey != "" {
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
