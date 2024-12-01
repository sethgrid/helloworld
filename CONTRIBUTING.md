### Restoration from scratch

helloworld.com runs on a digital ocean droplet running Ubuntu. It connects to a managed MySQL cluster. Locally, the service is managed by `systemctl` and logs are collected via `journalctl`. Secrets are in my Bitwarden.

See appendix for files.

Ensure the node only allows ssh over keys; disable passwords in `/etc/ssh/sshd_config`.

Get journalctl set up and:
```
sudo systemctl restart systemd-journald
```

Set up `/home/seth/projects/helloworld` (and `punicode` and `me` / personal site).
You can create `./bin/helloworld` via scp or you can download the repo and build locally

Get the services in place and run:
```
sudo systemctl daemon-reload
sudo systemctl restart helloworld.service # and punicode.service
```

Starting caddy:
```
sudo caddy stop && sudo caddy start --config Caddyfile
```

That should be enough to limp back to life.

Grafana UI can be ran from your local via a tunnel. However, for production db metrics, we have to have a
digital ocean personal access token with full db access. Use this guide to set up db monitoring but as of yet I am having trouble getting it working; maybe later:
https://www.digitalocean.com/community/tutorials/monitoring-mysql-and-mariadb-droplets-using-prometheus-mysql-exporter


and for getting set up with migrations, make sure to install goose
https://github.com/pressly/goose



### Other Services

- 0Auth, user seth.ammons@gmail.com
- DigialOcean, user seth.ammons@gmail.com
- Namecheap, user seth.ammons@gmail.com

### Appendix

`/home/seth/projects/helloworld/settings.env`
```
{{ Contents from Bitwarden; note: no "export", just key=val }}
```

`~/.my.cnf` (contains secret)
```
[client]
user = doadmin
password = {{ SEE BITWARDEN }}
host = db-mysql-helloworld-do-user-619265-0.g.db.ondigitalocean.com
port = 25060
database = helloworld
ssl-ca = ~/ca-certificate.crt
ssl-mode = VERIFY_CA

```

`/etc/systemd/system/helloworld.service`
```
[Unit]
Description=helloworld.com Service
After=network.target

[Service]
EnvironmentFile=/home/seth/projects/helloworld/settings.env
ExecStart=/home/seth/projects/helloworld/bin/helloworld
WorkingDirectory=/home/seth/projects/helloworld
Restart=always
User=seth
Group=seth

[Install]
WantedBy=multi-user.target

```


`/etc/systemd/journald.conf`

```
#  This file is part of systemd.
#
#  systemd is free software; you can redistribute it and/or modify it
#  under the terms of the GNU Lesser General Public License as published by
#  the Free Software Foundation; either version 2.1 of the License, or
#  (at your option) any later version.
#
# Entries in this file show the compile time defaults.
# You can change settings by editing this file.
# Defaults can be restored by simply deleting this file.
#
# See journald.conf(5) for details.

[Journal]
#Storage=auto
#Compress=yes
#Seal=yes
#SplitMode=uid
#SyncIntervalSec=5m
#RateLimitIntervalSec=30s
#RateLimitBurst=1000
SystemMaxUse=100M
#SystemKeepFree=
SystemMaxFileSize=50M
#SystemMaxFiles=100
#RuntimeMaxUse=
#RuntimeKeepFree=
#RuntimeMaxFileSize=
#RuntimeMaxFiles=100
MaxRetentionSec=1month
#MaxFileSec=1month
#ForwardToSyslog=yes
#ForwardToKMsg=no
#ForwardToConsole=no
#ForwardToWall=yes
#TTYPath=/dev/console
#MaxLevelStore=debug
#MaxLevelSyslog=debug
#MaxLevelKMsg=notice
#MaxLevelConsole=info
#MaxLevelWall=emerg
#LineMax=48K

```

`~/.ssh/authorized_keys`
```
See BitWarden
```

`/home/seth/projects/Caddyfile`
```
www.helloworld.com, api.helloworld.com, helloworld.com {
	reverse_proxy localhost:6666
}

sethammons.com, www.sethammons.com {
	root * /home/seth/projects/me/site/public
	file_server
}

xn--pnicode-n2a.com {
	reverse_proxy localhost:5555
}
```

#### Other Services

Punicode

`/etc/systemd/system/punicode.service`
```
[Unit]
Description=Pünicode.com Service
After=network.target

[Service]
ExecStart=/home/seth/projects/punicode/bin/punicode
WorkingDirectory=/home/seth/projects/punicode
Restart=always
User=seth
Group=seth

[Install]
WantedBy=multi-user.target

```