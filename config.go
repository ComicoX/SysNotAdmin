package main

import (
	"encoding/json"
	"io/ioutil"
)

type Remote struct {
	Name     string `json:"name"`
	Host     string `json:"ip"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type Command struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Command    string `json:"command"`
	RemoteName string `json:"remote_name,omitempty"`
}

type User struct {
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	AllowedCommands []string `json:"allowed_commands"`
}

type Config struct {
	ServerIP   string `json:"server_ip"`
	ServerPort string `json:"server_port"`
	TLSCert    string `json:"tls_cert"`
	TLSKey     string `json:"tls_key"`

	SudoUser     string       `json:"sudo_user"`
	SudoPassword string       `json:"sudo_password"`
	Users        []User       `json:"users"`
	Remotes      []Remote     `json:"remotes"`
	Commands     []Command    `json:"commands"`
	Status       StatusConfig `json:"status"`
}

type StatusConfig struct {
	Local  []string            `json:"local"`
	Remote map[string][]string `json:"remote"`
}

var AppConfig Config

func LoadConfig(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &AppConfig)
	if err != nil {
		return err
	}

	return nil
}

func SaveConfig(filename string) error {
	out, err := json.MarshalIndent(AppConfig, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, out, 0600)
}
