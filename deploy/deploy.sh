#!/bin/bash
set -e

# ================================================
# Splitwise Deployment Script for AWS EC2
# ================================================
# Usage:
#   1. SSH into your EC2 instance
#   2. Clone the repo
#   3. Update deploy/.env.production with your values
#   4. Run: bash deploy/deploy.sh
# ================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "========================================="
echo "  Splitwise Deployment Script"
echo "========================================="

# ---- Step 1: Install Docker if not present ----
if ! command -v docker &> /dev/null; then
    echo "📦 Installing Docker..."
    sudo yum update -y 2>/dev/null || sudo apt-get update -y
    sudo yum install -y docker 2>/dev/null || sudo apt-get install -y docker.io
    sudo systemctl start docker
    sudo systemctl enable docker
    sudo usermod -aG docker $USER
    echo "✅ Docker installed. You may need to log out and back in."
fi

if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "📦 Installing Docker Compose..."
    sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" \
        -o /usr/local/bin/docker-compose
    sudo chmod +x /usr/local/bin/docker-compose
    echo "✅ Docker Compose installed."
fi

# ---- Step 2: Load environment ----
if [ ! -f "$SCRIPT_DIR/.env.production" ]; then
    echo "❌ Missing deploy/.env.production. Copy from .env.production and update values."
    exit 1
fi

source "$SCRIPT_DIR/.env.production"

# ---- Step 3: Build React frontend ----
echo "🔨 Building React frontend..."
cd "$PROJECT_ROOT/../splitwise_client_react"

# Create .env for Vite build
cat > .env.production << EOF
VITE_API_URL=${VITE_API_URL}
VITE_WS_URL=${VITE_WS_URL}
EOF

npm ci --production=false
npm run build

# Copy built files to where Nginx expects them
mkdir -p "$PROJECT_ROOT/splitwise_client_build"
cp -r dist/* "$PROJECT_ROOT/splitwise_client_build/"

echo "✅ Frontend built successfully."

# ---- Step 4: Get SSL cert (first time only) ----
DOMAIN=$(echo "$VITE_API_URL" | sed 's|https://||')

if [ ! -d "$SCRIPT_DIR/certbot/conf/live/$DOMAIN" ]; then
    echo "🔒 Getting SSL certificate for $DOMAIN..."
    
    # Start nginx temporarily for ACME challenge
    cd "$SCRIPT_DIR"
    
    # Use a temporary nginx config without SSL first
    cat > nginx_temp.conf << 'TEMPEOF'
server {
    listen 80;
    server_name _;
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    location / {
        return 200 'OK';
    }
}
TEMPEOF

    docker run -d --name temp_nginx -p 80:80 \
        -v "$SCRIPT_DIR/nginx_temp.conf:/etc/nginx/conf.d/default.conf" \
        -v "$SCRIPT_DIR/certbot/www:/var/www/certbot" \
        nginx:alpine

    # Get the certificate
    docker run --rm \
        -v "$SCRIPT_DIR/certbot/conf:/etc/letsencrypt" \
        -v "$SCRIPT_DIR/certbot/www:/var/www/certbot" \
        certbot/certbot certonly --webroot \
        -w /var/www/certbot \
        -d "$DOMAIN" \
        --email "admin@$DOMAIN" \
        --agree-tos \
        --no-eff-email

    docker stop temp_nginx && docker rm temp_nginx
    rm -f nginx_temp.conf

    echo "✅ SSL certificate obtained."
else
    echo "✅ SSL certificate already exists."
fi

# ---- Step 5: Update nginx.conf with actual domain ----
echo "📝 Updating nginx.conf with domain: $DOMAIN"
sed -i "s/your-domain.com/$DOMAIN/g" "$SCRIPT_DIR/nginx.conf"

# ---- Step 6: Start everything ----
echo "🚀 Starting all services..."
cd "$SCRIPT_DIR"
docker compose -f docker-compose.prod.yml up -d --build

echo ""
echo "========================================="
echo "  ✅ Deployment Complete!"
echo "========================================="
echo "  🌐 Frontend: https://$DOMAIN"
echo "  🔌 API:      https://$DOMAIN/api/"
echo "  🔗 WebSocket: wss://$DOMAIN/ws"
echo ""
echo "  📋 View logs: docker compose -f deploy/docker-compose.prod.yml logs -f"
echo "  🔄 Restart:   docker compose -f deploy/docker-compose.prod.yml restart"
echo "  🛑 Stop:      docker compose -f deploy/docker-compose.prod.yml down"
echo "========================================="
