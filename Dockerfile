# Multi-stage Docker build for Railway deployment
FROM golang:1.19-alpine AS go-builder

# Build WhatsApp Bridge
WORKDIR /app/whatsapp-bridge
COPY services/whatsapp-bridge/ .
RUN go mod tidy
RUN go build -o whatsapp-bridge .

# Python stage
FROM python:3.11-slim AS python-stage

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    postgresql-client \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python dependencies
COPY services/requirements.txt* ./
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

# Copy WhatsApp Bridge from Go builder
COPY --from=go-builder /app/whatsapp-bridge/whatsapp-bridge ./services/whatsapp-bridge/

# Copy config (credentials will be provided via environment variables)
COPY .env.template ./ 2>/dev/null || true

# Create directories for persistent data
RUN mkdir -p /app/store /app/logs

# Note: WhatsApp session files will be created at runtime
# (not included in repo for security reasons)

# Set permissions
RUN chmod +x ./services/whatsapp-bridge/whatsapp-bridge

# Expose WhatsApp Bridge port
EXPOSE ${PORT:-8080}

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD curl -f http://localhost:${PORT:-8080}/health || exit 1

# Start script that runs all services
COPY start-services.sh ./
RUN chmod +x start-services.sh

CMD ["./start-services.sh"]