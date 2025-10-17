# Railway deployment with Go compilation
FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    postgresql-client \
    && rm -rf /var/lib/apt/lists/*

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

# Copy Python services and pre-built binary
COPY services/ ./services/

# Use pre-built WhatsApp bridge binary (temporary fix for dependency issues)
RUN chmod +x ./services/whatsapp-bridge/whatsapp-bridge

# Create directories for persistent data
RUN mkdir -p /app/store /app/logs

# Expose WhatsApp Bridge port
EXPOSE ${PORT:-8080}

# Start script
COPY fix_credentials.py ./
COPY start-services.sh ./
RUN chmod +x start-services.sh

# Force rebuild for clean state
CMD ["./start-services.sh"]
