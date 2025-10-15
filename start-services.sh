#!/bin/bash

# Railway Service Startup Script for Velo Test WhatsApp Monitor
# Simplified version using persistent volumes instead of environment variables

set -e

echo "🚀 Starting Velo Test WhatsApp Monitor Services on Railway..."
echo "=============================================================="

# Set up environment variables
export WHATSAPP_DB_PATH="./store/messages.db"
# Note: Railway sets GOOGLE_APPLICATION_CREDENTIALS=/app/credentials.json  
# We need to update it to point to our actual credentials file location
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/credentials.json"
# Ensure Google Sheets ID is available for Drop Monitor (should already be set by Railway)
export GOOGLE_SHEETS_ID="${GOOGLE_SHEETS_ID:-1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk}"

# Create credentials.json from environment variable
if [ -n "$GOOGLE_CREDENTIALS_JSON" ]; then
    echo "🔐 Creating credentials.json from environment variable..."
    
    # Create credentials at both locations to ensure compatibility
    echo "$GOOGLE_CREDENTIALS_JSON" > ./credentials.json
    
    # Also create at Railway's expected location if /app directory exists
    if [ -w /app ] || mkdir -p /app 2>/dev/null; then
        echo "$GOOGLE_CREDENTIALS_JSON" > /app/credentials.json
        echo "✅ Credentials file created at /app/credentials.json"
    else
        echo "✅ Credentials file created at $(pwd)/credentials.json"
        echo "🔍 Note: Cannot write to /app, using current directory"
    fi
    
    echo "🔍 Environment variables set:"
    echo "  GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS"
    echo "  GOOGLE_SHEETS_ID=$GOOGLE_SHEETS_ID"
else
    echo "⚠️  Warning: GOOGLE_CREDENTIALS_JSON environment variable not set"
fi

# WhatsApp Session Management - Persistent Volume Approach
echo "📱 Checking WhatsApp session storage..."

# Create required directories
mkdir -p ./store ./logs

# Check if session already exists in persistent volume
if [ -d "./store" ] && [ "$(ls -A ./store/*.db 2>/dev/null)" ]; then
    echo "✅ Found existing WhatsApp session in persistent storage"
    echo "🔄 WhatsApp will connect automatically (no QR code needed)"
    echo "📂 Session files:"
    ls -la ./store/ | grep -E '\.(db|json)$' || echo "   No session files found"
else
    echo "ℹ️  No existing WhatsApp session found"
    echo "📱 WhatsApp will prompt for QR code on first connection"
    echo "💾 Session will be automatically saved to persistent storage for future deployments"
fi

echo "🏠 Session storage directory: ./store"
echo "💡 After QR code scan, this session will persist across all Railway deployments"

# Start WhatsApp Bridge in the background
echo "📱 Starting WhatsApp Bridge..."
cd ./services/whatsapp-bridge

# Create symlink so WhatsApp bridge can find credentials at relative path
ln -sf ../../credentials.json ./credentials.json

# Check if this is first time setup (no session files)
if [ ! -f "../../store/whatsapp.db" ]; then
    echo "🔐 First time setup - WhatsApp authentication required"
    echo "📱 QR CODE WILL APPEAR BELOW IN THESE LOGS:"
    echo "================== QR CODE OUTPUT START =================="
fi

# Start WhatsApp bridge with output to both logs AND console (so QR appears in Railway logs)
./whatsapp-bridge 2>&1 | tee ../../logs/whatsapp-bridge.log &
BRIDGE_PID=$!
echo "✅ WhatsApp Bridge started (PID: $BRIDGE_PID)"

# Give WhatsApp bridge time to show QR code
echo "⏳ Waiting for WhatsApp Bridge to show QR code..."
sleep 15

# Check if QR code appeared in logs and display it
echo "================== CHECKING FOR QR CODE =================="
if [ -f "../../logs/whatsapp-bridge.log" ]; then
    echo "🔍 Checking WhatsApp bridge logs for QR code..."
    
    # Look for QR code patterns in the log file
    if grep -q "QR code" ../../logs/whatsapp-bridge.log; then
        echo "📱 QR CODE FOUND IN LOGS - DISPLAYING NOW:"
        echo "=" | tr '=' '=' | head -c 60; echo
        
        # Extract and display the QR code section
        sed -n '/QR code/,/Connected\|Failed\|Error/p' ../../logs/whatsapp-bridge.log
        
        echo ""; echo "=" | tr '=' '=' | head -c 60; echo
        echo "📱 SCAN THE QR CODE ABOVE WITH YOUR WHATSAPP APP!"
        echo "💾 Once connected, your session will be saved automatically"
        echo "🚀 Future deployments will connect automatically (no QR needed)"
    else
        echo "⚠️  QR code not found in logs yet. It may appear later."
        echo "🔍 First 50 lines of bridge log:"
        head -50 ../../logs/whatsapp-bridge.log || echo "Log file empty"
    fi
else
    echo "⚠️  WhatsApp bridge log file not created yet"
fi
echo "================== QR CODE CHECK COMPLETE =================="

# Wait for WhatsApp Bridge to be ready
echo "⏳ Waiting for WhatsApp Bridge to initialize..."
sleep 10

# Check if bridge is healthy
for i in {1..10}; do
    if curl -f http://localhost:${PORT:-8080}/ >/dev/null 2>&1; then
        echo "✅ WhatsApp Bridge is responding"
        break
    fi
    echo "⏳ Waiting for bridge to respond... ($i/10)"
    sleep 5
done

# Start Drop Monitor
echo "🔍 Starting Drop Monitor..."
cd ../../
python3 services/realtime_drop_monitor.py > ./logs/drop-monitor.log 2>&1 &
MONITOR_PID=$!
echo "✅ Drop Monitor started (PID: $MONITOR_PID)"

# Start QA Feedback Service
echo "💬 Starting QA Feedback Service..."
python3 services/smart_qa_feedback.py --interval 120 > ./logs/qa-feedback.log 2>&1 &
QA_PID=$!
echo "✅ QA Feedback Service started (PID: $QA_PID)"

echo ""
echo "🎉 All services started successfully!"
echo "📊 Service PIDs:"
echo "  - WhatsApp Bridge: $BRIDGE_PID"
echo "  - Drop Monitor: $MONITOR_PID"  
echo "  - QA Feedback: $QA_PID"
echo ""
echo "💡 IMPORTANT: After QR code scan, WhatsApp session is saved to persistent volume"
echo "🚀 Future Railway deployments will automatically connect (no QR needed)"
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
        cd ./services/whatsapp-bridge
        ./whatsapp-bridge > ../../logs/whatsapp-bridge.log 2>&1 &
        BRIDGE_PID=$!
        echo "🔄 WhatsApp Bridge restarted (PID: $BRIDGE_PID)"
        cd ../../
    fi
    
    # Check Drop Monitor  
    if ! is_running $MONITOR_PID; then
        echo "❌ Drop Monitor crashed, restarting..."
        python3 services/realtime_drop_monitor.py > ./logs/drop-monitor.log 2>&1 &
        MONITOR_PID=$!
        echo "🔄 Drop Monitor restarted (PID: $MONITOR_PID)"
    fi
    
    # Check QA Feedback
    if ! is_running $QA_PID; then
        echo "❌ QA Feedback crashed, restarting..."
        python3 services/smart_qa_feedback.py --interval 120 > ./logs/qa-feedback.log 2>&1 &
        QA_PID=$!
        echo "🔄 QA Feedback restarted (PID: $QA_PID)"
    fi
    
    # Wait before next check
    sleep 30
done