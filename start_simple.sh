#!/bin/bash

# Simple Startup Script for Velo Test
# No complexity - just start services in background with clear logging

set -e

echo "🚀 Starting Velo Test Services (Simple Mode)"
echo "=========================================="

# Load environment
source .env

# Create logs directory
mkdir -p logs

# Function to start service with proper logging
start_service() {
    local name=$1
    local command=$2
    local log_file=$3

    echo "🔧 Starting $name..."
    eval "$command" > "$log_file" 2>&1 &
    local pid=$!
    echo $pid > "logs/${name}.pid"
    echo "✅ $name started (PID: $pid, Log: $log_file)"
    sleep 2

    # Verify it's running
    if kill -0 $pid 2>/dev/null; then
        echo "✅ $name confirmed running"
    else
        echo "❌ $name failed to start - check $log_file"
        return 1
    fi
}

echo ""
echo "📱 Step 1: Starting WhatsApp Bridge..."
start_service "whatsapp_bridge" "cd services/whatsapp-bridge && go run main.go" "logs/whatsapp_bridge.log"

echo ""
echo "🔍 Step 2: Starting Drop Monitor..."
start_service "drop_monitor" "source .venv/bin/activate && python3 services/realtime_drop_monitor.py" "logs/drop_monitor.log"

echo ""
echo "💬 Step 3: Starting QA Feedback Communicator..."
start_service "qa_feedback" "source .venv/bin/activate && python3 services/qa_feedback_communicator.py" "logs/qa_feedback.log"

echo ""
echo "✅ Step 4: Starting Done Message Detector..."
start_service "done_detector" "source .venv/bin/activate && python3 services/done_message_detector.py" "logs/done_detector.log"

echo ""
echo "🎉 All services started!"
echo ""
echo "📊 Service Status:"
echo "=================="
for service in whatsapp_bridge drop_monitor qa_feedback done_detector; do
    if [ -f "logs/${service}.pid" ]; then
        pid=$(cat "logs/${service}.pid")
        if kill -0 $pid 2>/dev/null; then
            echo "✅ $service: RUNNING (PID: $pid)"
        else
            echo "❌ $service: STOPPED"
        fi
    else
        echo "❌ $service: NOT STARTED"
    fi
done

echo ""
echo "📋 Quick Commands:"
echo "• View logs: tail -f logs/[service].log"
echo "• Stop all: pkill -f 'go run\|realtime_drop_monitor\|qa_feedback\|done_message_detector'"
echo "• Check ports: netstat -tlnp | grep :808"

echo ""
echo "🎯 Next Steps for Testing:"
echo "1. Scan WhatsApp QR code (check logs/whatsapp_bridge.log)"
echo "2. Test by posting 'DR9999999' in Velo Test WhatsApp group"
echo "3. Monitor logs for activity"
echo "4. Check Google Sheets for new entries"