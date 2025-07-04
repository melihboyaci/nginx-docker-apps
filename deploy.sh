#!/bin/bash

# Deployment script for Ubuntu server
echo "🚀 Starting deployment..."

# Create necessary directories
sudo mkdir -p /opt/chat-apps
sudo mkdir -p /opt/chat-apps/logs

# Stop existing containers if running
echo "🛑 Stopping existing containers..."
sudo docker-compose down 2>/dev/null || true

# Copy files to server directory
echo "📁 Copying application files..."
sudo cp -r . /opt/chat-apps/
cd /opt/chat-apps

# Set permissions
sudo chown -R $USER:$USER /opt/chat-apps
chmod +x deploy.sh

# Create nginx logs directory
sudo mkdir -p ./nginx/logs
sudo chmod 755 ./nginx/logs

# Build and start containers
echo "🔨 Building and starting containers..."
sudo docker-compose build --no-cache
sudo docker-compose up -d

# Wait for services to start
echo "⏳ Waiting for services to start..."
sleep 10

# Check container status
echo "📊 Container status:"
sudo docker-compose ps

# Check logs
echo "📋 Recent logs:"
sudo docker-compose logs --tail=20

# Setup firewall rules
echo "🔒 Setting up firewall..."
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 22/tcp
sudo ufw --force enable

# Setup log rotation
echo "📝 Setting up log rotation..."
sudo tee /etc/logrotate.d/chat-apps > /dev/null <<EOF
/opt/chat-apps/nginx/logs/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    sharedscripts
    postrotate
        docker exec nginx-proxy nginx -s reload 2>/dev/null || true
    endscript
}
EOF

# Setup systemd service for auto-restart
echo "⚙️ Setting up systemd service..."
sudo tee /etc/systemd/system/chat-apps.service > /dev/null <<EOF
[Unit]
Description=Chat Applications
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/chat-apps
ExecStart=/usr/bin/docker-compose up -d
ExecStop=/usr/bin/docker-compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable chat-apps.service

echo "✅ Deployment completed!"
echo "🌐 Applications will be available at:"
echo "   - Chat App: https://melihboyaci.xyz"
echo "   - Question App: https://melihboyaci.online"
echo ""
echo "📊 To monitor:"
echo "   - Logs: sudo docker-compose logs -f"
echo "   - Status: sudo docker-compose ps"
echo "   - Restart: sudo systemctl restart chat-apps"
