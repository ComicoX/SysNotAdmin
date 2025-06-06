SysNotAdmin is a secure web-based system control panel written in Go.

It allows authorized users to run selected system commands and view service statuses from a simple web interface — without giving them shell access.

    ✅ Web UI with login

    ✅ Multiple users, per-command permissions

    ✅ Local and remote (SSH) commands

    ✅ Service status (local & remote), with uptime

    ✅ Live refresh (AJAX) every 15 min

    ✅ Manual Refresh Status button

    ✅ IP ban (jail) after 3 failed login attempts (stored in jail.txt)

    ✅ HTTPS support

    ✅ Runs as systemd service

Example config.json
```json
{
  "server_ip": "0.0.0.0",
  "server_port": "8443",
  "tls_cert": "cert.pem",
  "tls_key": "key.pem",
  "sudo_user": "yourlinuxuser",
  "sudo_password": "yourSudoPassword",

  "users": [
    {
      "username": "admin",
      "password": "adminpass",
      "allowed_commands": ["*"]
    },
    {
      "username": "limiteduser",
      "password": "limitedpass",
      "allowed_commands": ["Restart Plex", "Restart Web Nginx"]
    }
  ],

  "remotes": [
    {
      "name": "Web Server",
      "ip": "192.168.1.100",
      "user": "remoteuser1",
      "password": "remotepassword1"
    },
    {
      "name": "Database Server",
      "ip": "192.168.1.200",
      "user": "remoteuser2",
      "password": "remotepassword2"
    }
  ],

  "commands": [
    {
      "name": "Restart Plex",
      "type": "local",
      "command": "systemctl restart plexmediaserver"
    },
    {
      "name": "Restart Nginx",
      "type": "local",
      "command": "systemctl restart nginx && systemctl restart php8.2-fpm"
    },
    {
      "name": "Restart Web Nginx",
      "type": "remote",
      "remote_name": "Web Server",
      "command": "sudo systemctl restart nginx && sudo systemctl restart php8.2-fpm"
    },
    {
      "name": "Restart Database",
      "type": "remote",
      "remote_name": "Database Server",
      "command": "sudo systemctl restart mysql"
    }
  ],

  "status": {
    "local": ["plexmediaserver", "nginx"],
    "remote": {
      "Web Server": ["nginx"],
      "Database Server": ["mysql"]
    }
  }
}
```

Deploymment
====

Example Systemd Service
```
/etc/systemd/system/sysnotadmin.service:

[Unit]
Description=SysNotAdmin Service
After=network.target

[Service]
ExecStart=/opt/sysnotadmin/sysnotadmin
WorkingDirectory=/opt/sysnotadmin
User=sysnotadmin
Group=sysnotadmin
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

    Build:
    go build -o sysnotadmin

    Create dedicated user:
    sudo useradd -r -s /bin/false sysnotadmin

    Prepare directory:
    sudo mkdir -p /opt/sysnotadmin
    sudo cp sysnotadmin /opt/sysnotadmin/
    sudo chmod +x sysnotadmin
    sudo cp config.json /opt/sysnotadmin/
    sudo cp jail.txt /opt/sysnotadmin/
    sudo cp -r templates /opt/sysnotadmin/
    
    Set permissions
    sudo chown -R sysnotadmin:sysnotadmin /opt/sysnotadmin

    Install systemd service
    sudo cp sysnotadmin.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable sysnotadmin.service
    sudo systemctl start sysnotadmin.service

    View status
    sudo systemctl status sysnotadmin.service

    View logs
    journalctl -u sysnotadmin.service -f

