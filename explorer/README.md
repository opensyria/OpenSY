# OpenSyria Block Explorer | مستكشف سلسلة الكتل

A lightweight, bilingual (Arabic/English) block explorer for the OpenSyria blockchain.

مستكشف بسيط وخفيف لسلسلة كتل سوريا المفتوحة، يدعم العربية والإنجليزية.

## Features | الميزات

- 🌐 **Bilingual** - Full Arabic (RTL) and English support
- 🚀 **Lightweight** - No database required, connects directly to RPC
- 📱 **Responsive** - Works on desktop and mobile
- 🎨 **Syrian themed** - Colors inspired by the Syrian flag
- 🔍 **Search** - Find blocks, transactions, and addresses

## Quick Start | البدء السريع

### Prerequisites | المتطلبات

- Node.js 18+
- Running OpenSyria node with RPC enabled

### Installation | التثبيت

```bash
cd explorer
npm install
cp .env.example .env
# Edit .env with your RPC credentials
npm start
```

### Configuration | الإعدادات

Edit `.env`:

```ini
# Node RPC Connection
RPC_HOST=127.0.0.1
RPC_PORT=9632
RPC_USER=opensyria
RPC_PASSWORD=your_password

# Explorer
PORT=3000
DEFAULT_LANG=ar   # ar or en
```

## Deployment | النشر

### On the same server as the node

```bash
# SSH to server
ssh -i ~/.ssh/key.pem user@server

# Clone explorer or copy files
cd /opt/opensyria
git pull  # or copy explorer folder

# Install and run
cd explorer
npm install
cp .env.example .env
nano .env  # Edit RPC credentials

# Start with PM2 (recommended)
npm install -g pm2
pm2 start server.js --name opensyria-explorer
pm2 save
pm2 startup
```

### With Nginx reverse proxy

```nginx
server {
    listen 80;
    server_name explorer.opensyria.net;
    
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### With SSL (Let's Encrypt)

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d explorer.opensyria.net
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/status` | Network status |
| `GET /api/block/:hash` | Block details |
| `GET /api/tx/:txid` | Transaction details |

## Screenshots

### Arabic (RTL)
![Arabic Interface](docs/screenshot-ar.png)

### English (LTR)
![English Interface](docs/screenshot-en.png)

## Development

```bash
npm run dev  # With auto-reload
```

## License

MIT - Free to use and modify.

---

**سوريا حرة** 🇸🇾 **Free Syria**
