# Railway deployment with Go compilation
FROM python:3.11-slim

# Install system dependencies including Go
RUN apt-get update && apt-get install -y \
    curl \
    postgresql-client \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.23 (required by go.mod)
RUN wget -O go.tar.gz https://go.dev/dl/go1.23.0.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go.tar.gz \
    && rm go.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

WORKDIR /app

# Install Python dependencies
RUN pip install --no-cache-dir \
    psycopg2-binary \
    openai \
    google-auth \
    google-auth-oauthlib \
    google-auth-httplib2 \
    google-api-python-client \
    requests

# Copy Python services
COPY services/ ./services/

# Build WhatsApp bridge binary from source
WORKDIR /app/services/whatsapp-bridge
RUN go mod download && go build -o whatsapp-bridge main.go
RUN chmod +x whatsapp-bridge
WORKDIR /app

# Create directories for persistent data
RUN mkdir -p /app/store /app/logs

# Expose WhatsApp Bridge port
EXPOSE ${PORT:-8080}

# Start script
COPY start-services.sh ./
RUN chmod +x start-services.sh

CMD ["./start-services.sh"]
