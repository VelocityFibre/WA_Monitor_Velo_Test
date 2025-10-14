#!/bin/bash

# Railway Service Startup Script for Velo Test WhatsApp Monitor
# Starts all services in the correct order with proper environment setup

set -e

echo "🚀 Starting Velo Test WhatsApp Monitor Services on Railway..."
echo "=============================================================="

# Set up environment variables
export WHATSAPP_DB_PATH="/app/store/messages.db"
export GOOGLE_APPLICATION_CREDENTIALS="/app/credentials.json"

# Create credentials.json from environment variable
if [ -n "$GOOGLE_CREDENTIALS_JSON" ]; then
    echo "🔐 Creating credentials.json from environment variable..."
    echo "$GOOGLE_CREDENTIALS_JSON" > /app/credentials.json
    echo "✅ Credentials file created"
else
    echo "⚠️  Warning: GOOGLE_CREDENTIALS_JSON environment variable not set"
fi

# Create required directories
mkdir -p /app/store /app/logs

# Start WhatsApp Bridge in the background
echo "📱 Starting WhatsApp Bridge..."
cd /app/services/whatsapp-bridge
./whatsapp-bridge > /app/logs/whatsapp-bridge.log 2>&1 &
BRIDGE_PID=$!
echo "✅ WhatsApp Bridge started (PID: $BRIDGE_PID)"

# Wait for WhatsApp Bridge to be ready
echo "⏳ Waiting for WhatsApp Bridge to initialize..."
sleep 10

# Check if bridge is healthy
for i in {1..10}; do
    if curl -f http://localhost:${PORT:-8080}/health >/dev/null 2>&1; then
        echo "✅ WhatsApp Bridge is healthy"
        break
    fi
    echo "⏳ Waiting for bridge health check... ($i/10)"
    sleep 5
done

# Start Drop Monitor
echo "🔍 Starting Drop Monitor..."
cd /app
python3 services/realtime_drop_monitor.py > /app/logs/drop-monitor.log 2>&1 &
MONITOR_PID=$!
echo "✅ Drop Monitor started (PID: $MONITOR_PID)"

# Start QA Feedback Service
echo "💬 Starting QA Feedback Service..."
python3 services/smart_qa_feedback.py --interval 120 > /app/logs/qa-feedback.log 2>&1 &
QA_PID=$!
echo "✅ QA Feedback Service started (PID: $QA_PID)"

echo ""
echo "🎉 All services started successfully!"
echo "📊 Service PIDs:"
echo "  - WhatsApp Bridge: $BRIDGE_PID"
echo "  - Drop Monitor: $MONITOR_PID"  
echo "  - QA Feedback: $QA_PID"
echo ""
echo "📋 Monitoring services..."

# Function to check if a process is running
is_running() {
    kill -0 "$1" 2>/dev/null
}

# Monitor all services and restart if needed
while true; do
    # Check WhatsApp Bridge
    if ! is_running $BRIDGE_PID; then
        echo "❌ WhatsApp Bridge crashed, restarting..."
        cd /app/services/whatsapp-bridge
        ./whatsapp-bridge > /app/logs/whatsapp-bridge.log 2>&1 &
        BRIDGE_PID=$!
        echo "🔄 WhatsApp Bridge restarted (PID: $BRIDGE_PID)"
    fi
    
    # Check Drop Monitor  
    if ! is_running $MONITOR_PID; then
        echo "❌ Drop Monitor crashed, restarting..."
        cd /app
        python3 services/realtime_drop_monitor.py > /app/logs/drop-monitor.log 2>&1 &
        MONITOR_PID=$!
        echo "🔄 Drop Monitor restarted (PID: $MONITOR_PID)"
    fi
    
    # Check QA Feedback
    if ! is_running $QA_PID; then
        echo "❌ QA Feedback crashed, restarting..."
        python3 services/smart_qa_feedback.py --interval 120 > /app/logs/qa-feedback.log 2>&1 &
        QA_PID=$!
        echo "🔄 QA Feedback restarted (PID: $QA_PID)"
    fi
    
    # Wait before next check
    sleep 30
done