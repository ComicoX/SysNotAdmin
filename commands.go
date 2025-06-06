package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"golang.org/x/crypto/ssh"
)

func RunHandler(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)
	user := GetUser(session.Username)

	if r.Method == "POST" {
		cmdName := r.FormValue("name")
		if !IsCommandAllowed(user, cmdName) {
			AppLogger.Printf("FORBIDDEN COMMAND ATTEMPT: user=%s cmd=%s IP=%s", user.Username, cmdName, r.RemoteAddr)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		AppLogger.Printf("COMMAND RUN: user=%s cmd=%s IP=%s", user.Username, cmdName, r.RemoteAddr)

		for _, c := range AppConfig.Commands {
			if c.Name == cmdName {
				if c.Type == "local" {
					runLocalCommand(c.Command)
				} else if c.Type == "remote" {
					runRemoteCommand(c)
				}
				break
			}
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func IsCommandAllowed(user *User, cmdName string) bool {
	if user == nil {
		return false
	}
	for _, allowed := range user.AllowedCommands {
		if allowed == "*" || allowed == cmdName {
			return true
		}
	}
	return false
}

func runLocalCommand(cmdStr string) {
	go func() {
		fullCmd := fmt.Sprintf("echo %s | sudo -S %s", AppConfig.SudoPassword, cmdStr)
		AppLogger.Printf("[LOCAL] Executing: %s", fullCmd)

		cmd := exec.Command("bash", "-c", fullCmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			AppLogger.Printf("[LOCAL] ERROR: %v | Output: %s", err, output)
		} else {
			AppLogger.Printf("[LOCAL] Output: %s", output)
		}
	}()
}

func runRemoteCommand(c Command) {
	remote := findRemoteByName(c.RemoteName)
	if remote == nil {
		AppLogger.Printf("[REMOTE] ERROR: Remote not found: %s", c.RemoteName)
		return
	}

	go func() {
		AppLogger.Printf("[REMOTE] Connecting to %s@%s", remote.User, remote.Host)

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
			AppLogger.Printf("[REMOTE] SSH connection FAILED: %v", err)
			return
		}
		defer client.Close()

		session, err := client.NewSession()
		if err != nil {
			AppLogger.Printf("[REMOTE] Failed to create session: %v", err)
			return
		}
		defer session.Close()

		AppLogger.Printf("[REMOTE] Running command on %s: %s", remote.Host, c.Command)
		output, err := session.CombinedOutput(c.Command)
		if err != nil {
			AppLogger.Printf("[REMOTE] ERROR: %v | Output: %s", err, output)
		} else {
			AppLogger.Printf("[REMOTE] Output: %s", output)
		}
	}()
}

func findRemoteByName(name string) *Remote {
	for _, r := range AppConfig.Remotes {
		if r.Name == name {
			return &r
		}
	}
	return nil
}
