package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const JailFile = "jail.txt"
const JailBanThreshold = 3
const JailWindow = 24 * time.Hour

var JailData = struct {
	sync.RWMutex
	FailedLogins map[string][]time.Time
	BannedIPs    map[string]bool
}{
	FailedLogins: make(map[string][]time.Time),
	BannedIPs:    make(map[string]bool),
}

func LoadJail() {
	JailData.Lock()
	defer JailData.Unlock()

	file, err := os.Open(JailFile)
	if err != nil {
		AppLogger.Printf("[JAIL] No jail file found, starting fresh")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip != "" {
			JailData.BannedIPs[ip] = true
		}
	}

	AppLogger.Printf("[JAIL] Loaded %d banned IPs", len(JailData.BannedIPs))
}

func SaveJail() {
	JailData.Lock()
	defer JailData.Unlock()

	file, err := os.Create(JailFile)
	if err != nil {
		AppLogger.Printf("[JAIL] ERROR writing jail file: %v", err)
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for ip := range JailData.BannedIPs {
		fmt.Fprintln(writer, ip)
	}
	writer.Flush()

	AppLogger.Printf("[JAIL] Saved %d banned IPs", len(JailData.BannedIPs))
}

func IsIPBanned(ip string) bool {
	JailData.RLock()
	defer JailData.RUnlock()

	return JailData.BannedIPs[ip]
}

func RecordFailedLogin(ip string) {
	now := time.Now()

	// Clean old entries
	history := JailData.FailedLogins[ip]
	var newHistory []time.Time
	for _, t := range history {
		if now.Sub(t) <= JailWindow {
			newHistory = append(newHistory, t)
		}
	}

	// Add new fail
	newHistory = append(newHistory, now)
	JailData.FailedLogins[ip] = newHistory

	AppLogger.Printf("[JAIL] Failed login from %s, count=%d", ip, len(newHistory))

	// Ban if over threshold
	if len(newHistory) >= JailBanThreshold && !JailData.BannedIPs[ip] {
		JailData.BannedIPs[ip] = true
		AppLogger.Printf("[JAIL] BANNED IP: %s", ip)
		SaveJail()
	}
}
