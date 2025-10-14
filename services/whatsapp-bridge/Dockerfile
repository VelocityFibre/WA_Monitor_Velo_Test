FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    sqlite3 \
    wget \
    iproute2 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the existing binary
COPY whatsapp-bridge .

# Create directories for data persistence
RUN mkdir -p /app/store /app/logs

# Expose WhatsApp Bridge port
EXPOSE 8080

# Volume for persistent WhatsApp session and message data
VOLUME ["/app/store", "/app/logs"]

# Health check (check if port 8080 is listening)
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD ss -tln | grep :8080 || exit 1

# Run the bridge
CMD ["./whatsapp-bridge"]