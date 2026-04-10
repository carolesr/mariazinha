# 🤖 Mariazinha — WhatsApp Events Bot

Event management bot for WhatsApp communities.
Built with Go + Meta Cloud API + SQLite + Claude AI (Haiku).

## How it works

```
User @mentions Mariazinha in a group
        ↓
Meta sends a POST to your webhook
        ↓
Claude AI parses the intent (join, leave, create, etc.)
        ↓
Bot executes the action in SQLite
        ↓
Meta API sends the reply back to the group
```

## Prerequisites

- Go 1.21+
- A Meta Business account with WhatsApp Cloud API configured
- An Anthropic API key (https://console.anthropic.com)
- A server with a public HTTPS URL (Oracle Cloud Free Tier recommended)

---

## 1. Meta Setup

### Create a Meta App

1. Go to https://developers.facebook.com
2. Click **My Apps → Create App**
3. Choose **Business** type
4. Add the **WhatsApp** product to the app

### Get your credentials

In your app dashboard under **WhatsApp → API Setup**:
- Copy the **Phone Number ID** → `META_PHONE_ID`
- Copy the **Temporary Access Token** → `META_ACCESS_TOKEN`

> For production, generate a **permanent System User token** via Meta Business Manager.
> Temporary tokens expire after 24h.

### Register the webhook

Under **WhatsApp → Configuration → Webhook**:
- **Callback URL**: `https://YOUR_SERVER_IP/webhook`
- **Verify Token**: any secret string you choose → `META_VERIFY_TOKEN`
- Subscribe to the **messages** field

Meta will send a GET request to verify the webhook — the bot handles this automatically.

---

## 2. Server Setup (Oracle Cloud Free Tier)

### Create the VM

1. Go to https://cloud.oracle.com → Compute → Instances → Create Instance
2. Shape: **VM.Standard.E2.1.Micro** (Always Free)
3. Image: Ubuntu 22.04
4. Generate or upload an SSH key
5. Note the public IP

### Open port 443 (HTTPS)

In Oracle Cloud: Networking → Virtual Cloud Networks → your VCN →
Security Lists → Add Ingress Rule:
- Source: `0.0.0.0/0`
- Port: `443`

Also open it in the OS:
```bash
sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT
```

### Install Go

```bash
ssh ubuntu@YOUR_IP
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

### Install Caddy (handles HTTPS automatically)

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install caddy
```

Configure Caddy (`/etc/caddy/Caddyfile`):
```
YOUR_DOMAIN_OR_IP {
    reverse_proxy localhost:8080
}
```

```bash
sudo systemctl enable caddy
sudo systemctl start caddy
```

### Deploy the bot

```bash
# On your local machine
scp -r mariazinha ubuntu@YOUR_IP:~/

# On the VM
cd ~/mariazinha
cp .env.example .env
nano .env   # fill in your values
mkdir -p data
go mod tidy
go build -o mariazinha ./cmd/bot
```

### Run with systemd

```bash
sudo nano /etc/systemd/system/mariazinha.service
```

```ini
[Unit]
Description=Mariazinha WhatsApp Bot
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/mariazinha
EnvironmentFile=/home/ubuntu/mariazinha/.env
ExecStart=/home/ubuntu/mariazinha/mariazinha
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable mariazinha
sudo systemctl start mariazinha
sudo systemctl status mariazinha

# Live logs
journalctl -u mariazinha -f
```

---

## 3. Configure the webhook in Meta

Once the server is running, go back to **Meta → WhatsApp → Configuration → Webhook**
and click **Verify and Save**. You should see `✅ webhook verified by Meta` in the logs.

---

## Usage

Mention Mariazinha in any group she's a member of:

```
@Mariazinha me coloca no karaoke de sábado
@Mariazinha quero sair do piquenique dia 28
@Mariazinha detalha o evento trilha
@Mariazinha lista os eventos da semana
@Mariazinha cria trilha sábado 8h no Parque Barigui, levar água, 15 vagas, gratuito
```

### Admin commands (ADMIN_PHONES only)

```
@Mariazinha cancela a noite de jogos
@Mariazinha remove a Ana do karaoke
@Mariazinha edita o karaoke, muda as vagas para 20
```

---

## Project structure

```
mariazinha/
├── cmd/bot/main.go              # Entry point
├── internal/
│   ├── ai/ai.go                 # Claude Haiku intent parsing
│   ├── config/config.go         # Environment config
│   ├── db/db.go                 # SQLite database layer
│   ├── handler/
│   │   ├── handler.go           # All command logic
│   │   └── format.go            # Bot reply templates (in Portuguese)
│   ├── meta/meta.go             # Meta Cloud API client (sends messages)
│   └── webhook/webhook.go       # HTTP server, webhook verification, payload parsing
├── data/                        # Auto-created
│   └── events.db
├── .env.example
├── Makefile
└── go.mod
```

---

## Manual database editing

```bash
# Via SSH
sqlite3 ~/mariazinha/data/events.db

# Download, edit visually, re-upload
scp ubuntu@YOUR_IP:~/mariazinha/data/events.db ./events.db
# edit with DB Browser for SQLite (https://sqlitebrowser.org)
scp ./events.db ubuntu@YOUR_IP:~/mariazinha/data/events.db
```

## Backup

```bash
# Add to crontab
crontab -e
0 3 * * * cp /home/ubuntu/mariazinha/data/events.db /home/ubuntu/backups/events_$(date +\%Y\%m\%d).db
```
