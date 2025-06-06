package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type ServiceStatus struct {
	Name   string
	Active bool
	Uptime string
}

var (
	StatusData = struct {
		sync.RWMutex
		Local  []ServiceStatus
		Remote map[string][]ServiceStatus
	}{
		Remote: make(map[string][]ServiceStatus),
	}
)

func StatusRefresher() {
	for {
		AppLogger.Println("AUTO STATUS REFRESH")
		refreshLocalStatus()
		refreshRemoteStatus()
		time.Sleep(30 * time.Minute)
	}
}

func refreshLocalStatus() {
	var statuses []ServiceStatus
	for _, svc := range AppConfig.Status.Local {
		active, uptime := checkLocalServiceStatus(svc)
		statuses = append(statuses, ServiceStatus{
			Name:   svc,
			Active: active,
			Uptime: uptime,
		})
	}
	StatusData.Lock()
	StatusData.Local = statuses
	StatusData.Unlock()
}

func refreshRemoteStatus() {
	StatusData.Lock()
	defer StatusData.Unlock()
	StatusData.Remote = make(map[string][]ServiceStatus)

	for remoteName, services := range AppConfig.Status.Remote {
		remote := findRemoteByName(remoteName)
		if remote == nil {
			AppLogger.Printf("[REMOTE STATUS] Remote not found: %s", remoteName)
			continue
		}

		AppLogger.Printf("[REMOTE STATUS] Refreshing remote: %s", remoteName)
		var statuses []ServiceStatus
		for _, svc := range services {
			active, uptime := checkRemoteServiceStatus(remote, svc)
			statuses = append(statuses, ServiceStatus{
				Name:   svc,
				Active: active,
				Uptime: uptime,
			})
		}
		StatusData.Remote[remoteName] = statuses
	}
}

func formatDuration(d time.Duration) string {
	if d.Hours() >= 24 {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	} else if d.Hours() >= 1 {
		return fmt.Sprintf("%.0fh %.0fm", d.Hours(), d.Minutes()-float64(int(d.Hours())*60))
	} else if d.Minutes() >= 1 {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
}

func checkLocalServiceStatus(service string) (bool, string) {
	// Check active status
	cmd := exec.Command("bash", "-c", fmt.Sprintf("systemctl is-active %s", service))
	output, err := cmd.CombinedOutput()
	active := err == nil && string(output) == "active\n"

	// Get timestamp
	tsCmd := exec.Command("bash", "-c", fmt.Sprintf("systemctl show %s --property=ActiveEnterTimestamp", service))
	tsOutput, _ := tsCmd.CombinedOutput()

	tsLine := string(tsOutput)
	tsLine = strings.TrimSpace(tsLine)
	tsLine = strings.TrimPrefix(tsLine, "ActiveEnterTimestamp=")

	uptimeStr := ""
	if tsLine != "" && active {
		// Parse timestamp
		parsedTime, err := time.Parse("Mon 2006-01-02 15:04:05 MST", tsLine)
		if err == nil {
			duration := time.Since(parsedTime)
			uptimeStr = formatDuration(duration)
		} else {
			uptimeStr = "unknown"
		}
	} else {
		uptimeStr = "---"
	}

	return active, "Uptime: " + uptimeStr
}

func checkRemoteServiceStatus(remote *Remote, service string) (bool, string) {
	configSSH := &ssh.ClientConfig{
		User: remote.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(remote.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", remote.Host+":22", configSSH)
	if err != nil {
		AppLogger.Printf("[REMOTE STATUS] SSH connection FAILED: remote=%s service=%s ERR=%v", remote.Name, service, err)
		return false, "SSH_FAILED"
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		AppLogger.Printf("[REMOTE STATUS] Session FAILED: remote=%s service=%s ERR=%v", remote.Name, service, err)
		return false, "SSH_FAILED"
	}
	defer session.Close()

	output, err := session.CombinedOutput(fmt.Sprintf("systemctl is-active %s", service))
	active := err == nil && string(output) == "active\n"

	// Get uptime
	session2, err := client.NewSession()
	if err != nil {
		AppLogger.Printf("[REMOTE STATUS] Session2 FAILED: remote=%s service=%s ERR=%v", remote.Name, service, err)
		return active, "SSH_FAILED"
	}
	tsOutput, err := session2.CombinedOutput(fmt.Sprintf("systemctl show %s --property=ActiveEnterTimestamp", service))
	session2.Close()

	tsLine := string(tsOutput)
	tsLine = strings.TrimSpace(tsLine)
	tsLine = strings.TrimPrefix(tsLine, "ActiveEnterTimestamp=")

	uptimeStr := ""
	if tsLine != "" && active {
		parsedTime, err := time.Parse("Mon 2006-01-02 15:04:05 MST", tsLine)
		if err == nil {
			duration := time.Since(parsedTime)
			uptimeStr = formatDuration(duration)
		} else {
			uptimeStr = "unknown"
		}
	} else if err != nil {
		uptimeStr = "SSH_FAILED"
	} else {
		uptimeStr = "---"
	}

	return active, "Uptime: " + uptimeStr
}

func StatusAPIHandler(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Build API payload
	StatusData.RLock()
	payload := struct {
		Local  []ServiceStatus
		Remote map[string][]ServiceStatus
	}{
		Local:  append([]ServiceStatus{}, StatusData.Local...),
		Remote: make(map[string][]ServiceStatus),
	}
	for k, v := range StatusData.Remote {
		payload.Remote[k] = append([]ServiceStatus{}, v...)
	}
	StatusData.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
