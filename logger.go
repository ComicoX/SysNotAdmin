package main

import (
	"io"
	"log"
	"os"
	"time"
)

var AppLogger *log.Logger

func InitLogger() {
	logFile, err := os.OpenFile("sysnotadmin.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	AppLogger = log.New(multiWriter, "", log.LstdFlags)
	AppLogger.Println("========== SysNotAdmin STARTED ==========")
	AppLogger.Printf("Time: %s", time.Now().Format(time.RFC1123))
}
