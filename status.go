package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
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

		AppLogger.Printf("[REMOTE STATUS] Connecting to remote: %s", remoteName)

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
			AppLogger.Printf("[REMOTE STATUS] SSH connection failed to %s: %v", remoteName, err)
			continue
		}
		defer client.Close()

		var statuses []ServiceStatus
		for _, svc := range services {
			active, uptime := checkRemoteServiceStatus(client, svc)
			statuses = append(statuses, ServiceStatus{
				Name:   svc,
				Active: active,
				Uptime: uptime,
			})
		}
		StatusData.Remote[remoteName] = statuses
	}
}

func checkLocalServiceStatus(service string) (bool, string) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("systemctl status %s", service))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, "Failed to get status"
	}

	status := string(output)
	active := false
	uptimeStr := "---"

	activeRe := regexp.MustCompile(`(?m)^\s*Active:\s+active \(running\).*; (.+?) ago`)
	match := activeRe.FindStringSubmatch(status)
	if len(match) == 2 {
		active = true
		uptimeStr = match[1]
	}

	return active, "Uptime: " + uptimeStr
}

func checkRemoteServiceStatus(client *ssh.Client, service string) (bool, string) {
	session, err := client.NewSession()
	if err != nil {
		return false, "SESSION_FAILED"
	}
	defer session.Close()

	output, err := session.CombinedOutput(fmt.Sprintf("systemctl status %s", service))
	if err != nil {
		return false, "Failed to get status"
	}

	status := string(output)
	active := false
	uptimeStr := "---"

	activeRe := regexp.MustCompile(`(?m)^\s*Active:\s+active \(running\).*; (.+?) ago`)
	match := activeRe.FindStringSubmatch(status)
	if len(match) == 2 {
		active = true
		uptimeStr = match[1]
	}

	return active, "Uptime: " + uptimeStr
}

func StatusAPIHandler(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	refreshLocalStatus()
	refreshRemoteStatus()

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
