#!/bin/bash

# Railway Service Startup Script for Velo Test WhatsApp Monitor
# Simplified version using persistent volumes instead of environment variables

set -e

echo "🚀 Starting Velo Test WhatsApp Monitor Services on Railway..."
echo "==============================================================="

# Verify WhatsApp Bridge binary exists (built by Dockerfile)
if [ ! -f "services/whatsapp-bridge/whatsapp-bridge" ]; then
    echo "❌ WhatsApp Bridge binary not found - Docker build may have failed"
    exit 1
else
    echo "✅ WhatsApp Bridge binary ready"
fi

# Set up environment variables  
# WhatsApp Bridge creates database in services/whatsapp-bridge/store/
export WHATSAPP_DB_PATH="$(pwd)/services/whatsapp-bridge/store/messages.db"
# Note: Railway sets GOOGLE_APPLICATION_CREDENTIALS=/app/credentials.json by default
# Since we can't write to /app, we'll override it to point to our actual file location
# We'll set this after creating the credentials file
export GOOGLE_SHEETS_ID="${GOOGLE_SHEETS_ID:-1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk}"

# Create credentials.json from environment variable
if [ -n "$GOOGLE_CREDENTIALS_JSON" ]; then
    echo "🔐 Creating credentials.json from environment variable..."
    
    # Create credentials file in current directory (this always works)
    CREDENTIALS_FILE="$(pwd)/credentials.json"
    
    # Debug: Show first few characters to understand format
    echo "🔍 First 50 chars of credentials: ${GOOGLE_CREDENTIALS_JSON:0:50}..."
    
    # Handle different JSON formats that Railway might use
    if [[ "$GOOGLE_CREDENTIALS_JSON" == "{"* ]]; then
        # Already valid JSON format
        echo "✅ Detected raw JSON format"
        echo "$GOOGLE_CREDENTIALS_JSON" > "$CREDENTIALS_FILE"
    elif [[ "$GOOGLE_CREDENTIALS_JSON" == *"\\n"* ]] || [[ "$GOOGLE_CREDENTIALS_JSON" == *"\\"* ]]; then
        # Escaped JSON - decode escape sequences
        echo "🔧 Detected escaped JSON, decoding..."
        printf '%b' "$GOOGLE_CREDENTIALS_JSON" > "$CREDENTIALS_FILE"
    else
        # Try as-is first
        echo "🔧 Trying as-is format..."
        echo "$GOOGLE_CREDENTIALS_JSON" > "$CREDENTIALS_FILE"
    fi
    
    # Validate that we created valid JSON
    if python3 -m json.tool "$CREDENTIALS_FILE" >/dev/null 2>&1; then
        echo "✅ Valid JSON credentials file created"
    else
        echo "❌ Invalid JSON, trying alternative approaches..."
        
        # Try removing outer quotes if present
        if [[ "$GOOGLE_CREDENTIALS_JSON" == '"'*'"' ]]; then
            echo "🔧 Removing outer quotes and trying again..."
            CLEANED_JSON="${GOOGLE_CREDENTIALS_JSON:1:-1}"
            printf '%b' "$CLEANED_JSON" > "$CREDENTIALS_FILE"
        fi
        
        # Final validation
        if ! python3 -m json.tool "$CREDENTIALS_FILE" >/dev/null 2>&1; then
            echo "❌ CRITICAL: Cannot create valid credentials file"
            echo "🔍 Raw environment variable content:"
            echo "$GOOGLE_CREDENTIALS_JSON" | head -c 200
            echo ""
            echo "⚠️  Google Sheets integration will not work until credentials are fixed"
        else
            echo "✅ Successfully created valid JSON after cleanup"
        fi
    fi
    
    # Override Railway's GOOGLE_APPLICATION_CREDENTIALS to point to our actual file
    export GOOGLE_APPLICATION_CREDENTIALS="$CREDENTIALS_FILE"
    
    echo "✅ Credentials file created at $CREDENTIALS_FILE"
    echo "🔍 Environment variables set:"
    echo "  GOOGLE_APPLICATION_CREDENTIALS=$GOOGLE_APPLICATION_CREDENTIALS"
    echo "  GOOGLE_SHEETS_ID=$GOOGLE_SHEETS_ID"
    echo "  WHATSAPP_DB_PATH=$WHATSAPP_DB_PATH"
    
    # Verify database file exists
    if [ -f "$WHATSAPP_DB_PATH" ]; then
        echo "✅ WhatsApp database found at $WHATSAPP_DB_PATH"
    else
        echo "⚠️  WhatsApp database NOT found at $WHATSAPP_DB_PATH"
        echo "🔍 Available database files:"
        find ./services/whatsapp-bridge/store/ -name "*.db" 2>/dev/null || echo "   No .db files found"
    fi
else
    echo "⚠️  Warning: GOOGLE_CREDENTIALS_JSON environment variable not set"
fi

# WhatsApp Session Management - Database Persistence Approach
echo "📱 Checking WhatsApp session storage..."

# Create required directories
mkdir -p ./store ./logs

# Check if database persistence is available (try both DATABASE_URL and NEON_DATABASE_URL)
DATABASE_CONNECTION_URL="${DATABASE_URL:-$NEON_DATABASE_URL}"
if [ -n "$DATABASE_CONNECTION_URL" ]; then
    echo "🔧 Database found - initializing session persistence system..."
    export DATABASE_URL="$DATABASE_CONNECTION_URL"
    python3 services/session_persistence.py init
    
    # Try to restore previous session from database backup
    echo "📂 Attempting to restore WhatsApp session from backup..."
    if python3 services/session_persistence.py restore; then
        # Check if we successfully restored session files
        if [ -f "./store/whatsapp.db" ] || [ -f "./services/whatsapp-bridge/session.json" ] || ls ./store/*.db >/dev/null 2>&1; then
            echo "✅ WhatsApp session restored from database backup"
            echo "🔄 WhatsApp should connect automatically (no QR code needed)"
            echo "📂 Restored session files:"
            ls -la ./store/ | grep -E '\.(db|json|key|crt)$' || echo "   Database files only"
        else
            echo "ℹ️  No previous session backup found in database"
            echo "📱 WhatsApp will prompt for QR code on first connection"
        fi
    else
        echo "⚠️  Session restore failed, starting fresh"
        echo "📱 WhatsApp will prompt for QR code on first connection"
    fi
else
    echo "⚠️  No DATABASE_URL found - session persistence disabled"
    echo "ℹ️  Sessions will not persist across deployments"
    echo "📱 WhatsApp will prompt for QR code on each deployment"
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

# Start Session Persistence Monitor only if database is available
if [ -n "$DATABASE_CONNECTION_URL" ]; then
    echo "💾 Starting Session Persistence Monitor..."
    python3 services/session_persistence.py monitor > ./logs/session-persistence.log 2>&1 &
    PERSIST_PID=$!
    echo "✅ Session Persistence Monitor started (PID: $PERSIST_PID)"
else
    echo "⚠️  Session Persistence Monitor disabled (no database)"
    PERSIST_PID=""
fi

echo ""
echo "🎉 All services started successfully!"
echo "📊 Service PIDs:"
echo "  - WhatsApp Bridge: $BRIDGE_PID"
echo "  - Drop Monitor: $MONITOR_PID"  
echo "  - QA Feedback: $QA_PID"
if [ -n "$PERSIST_PID" ]; then
    echo "  - Session Persistence: $PERSIST_PID"
else
    echo "  - Session Persistence: disabled (no database)"
fi
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
        
        # Backup session after restart (only if database available)
        if [ -n "$DATABASE_CONNECTION_URL" ]; then
            echo "💾 Backing up WhatsApp session after restart..."
            python3 services/session_persistence.py backup >/dev/null 2>&1 || echo "⚠️  Session backup failed"
        fi
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
    
    # Check Session Persistence Monitor (only if enabled)
    if [ -n "$PERSIST_PID" ] && ! is_running $PERSIST_PID; then
        echo "❌ Session Persistence Monitor crashed, restarting..."
        python3 services/session_persistence.py monitor > ./logs/session-persistence.log 2>&1 &
        PERSIST_PID=$!
        echo "🔄 Session Persistence Monitor restarted (PID: $PERSIST_PID)"
    fi
    
    # Wait before next check
    sleep 30
done