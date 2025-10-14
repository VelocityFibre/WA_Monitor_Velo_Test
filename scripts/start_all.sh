#!/bin/bash

# Velo Test - Start All Services
# This script starts all Velo Test services in the correct order

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project directory
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"

echo -e "${BLUE}🚀 Starting Velo Test Services...${NC}"
echo -e "${BLUE}Project Directory: $PROJECT_DIR${NC}"

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo -e "${RED}❌ .env file not found! Please copy .env.template to .env and configure it.${NC}"
    exit 1
fi

# Load environment variables
source .env

# Function to check if port is in use
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo -e "${YELLOW}⚠️  Port $port is already in use${NC}"
        return 1
    fi
    return 0
}

# Function to wait for service to be ready
wait_for_service() {
    local port=$1
    local service_name=$2
    local max_attempts=30
    local attempt=1

    echo -e "${YELLOW}⏳ Waiting for $service_name to be ready...${NC}"

    while [ $attempt -le $max_attempts ]; do
        if curl -s "http://localhost:$port/health" >/dev/null 2>&1 || nc -z localhost $port 2>/dev/null; then
            echo -e "${GREEN}✅ $service_name is ready!${NC}"
            return 0
        fi

        echo -e "${YELLOW}⏳ Attempt $attempt/$max_attempts: $service_name not ready yet...${NC}"
        sleep 2
        ((attempt++))
    done

    echo -e "${RED}❌ $service_name failed to start within expected time${NC}"
    return 1
}

# Create necessary directories
echo -e "${BLUE}📁 Creating necessary directories...${NC}"
mkdir -p logs docker-data/whatsapp-sessions docker-data/bridge-logs docker-data/monitor-logs

# Check Python virtual environment
if [ ! -d ".venv" ]; then
    echo -e "${YELLOW}⚠️  Python virtual environment not found. Creating...${NC}"
    python3 -m venv .venv
    source .venv/bin/activate
    pip install -r services/requirements.txt
else
    source .venv/bin/activate
fi

# Check Go dependencies
echo -e "${BLUE}📦 Checking Go dependencies...${NC}"
cd services/whatsapp-bridge
if [ ! -f "go.sum" ]; then
    echo -e "${YELLOW}⚠️  Go dependencies not found. Downloading...${NC}"
    go mod download
    go mod tidy
fi
cd ../..

# Start services in correct order

# 1. Start WhatsApp Bridge (Port 8080)
echo -e "${BLUE}🔗 Starting WhatsApp Bridge (Port 8080)...${NC}"
if ! check_port 8080; then
    echo -e "${RED}❌ Port 8080 is already in use. Please stop the conflicting service.${NC}"
    exit 1
fi

cd services/whatsapp-bridge
nohup go run main.go > ../logs/whatsapp_bridge.log 2>&1 &
WHATSAPP_PID=$!
echo $WHATSAPP_PID > ../logs/whatsapp_bridge.pid
cd ../..

echo -e "${GREEN}✅ WhatsApp Bridge started (PID: $WHATSAPP_PID)${NC}"

# Wait for WhatsApp Bridge to be ready
if ! wait_for_service 8080 "WhatsApp Bridge"; then
    echo -e "${RED}❌ WhatsApp Bridge failed to start. Check logs: logs/whatsapp_bridge.log${NC}"
    kill $WHATSAPP_PID 2>/dev/null || true
    exit 1
fi

# 2. Start Drop Monitor Service
echo -e "${BLUE}🔍 Starting Drop Monitor Service...${NC}"
nohup python3 services/realtime_drop_monitor.py > logs/drop_monitor.log 2>&1 &
DROP_MONITOR_PID=$!
echo $DROP_MONITOR_PID > logs/drop_monitor.pid
echo -e "${GREEN}✅ Drop Monitor started (PID: $DROP_MONITOR_PID)${NC}"

# 3. Start QA Feedback Communicator
echo -e "${BLUE}💬 Starting QA Feedback Communicator...${NC}"
nohup python3 services/qa_feedback_communicator.py > logs/qa_feedback.log 2>&1 &
QA_FEEDBACK_PID=$!
echo $QA_FEEDBACK_PID > logs/qa_feedback.pid
echo -e "${GREEN}✅ QA Feedback Communicator started (PID: $QA_FEEDBACK_PID)${NC}"

# 4. Start Done Message Detector
echo -e "${BLUE}✅ Starting Done Message Detector...${NC}"
nohup python3 services/done_message_detector.py > logs/done_detector.log 2>&1 &
DONE_DETECTOR_PID=$!
echo $DONE_DETECTOR_PID > logs/done_detector.pid
echo -e "${GREEN}✅ Done Message Detector started (PID: $DONE_DETECTOR_PID)${NC}"

# 5. Start Velo Test Service (if available)
if [ -f "services/velo_test_service.py" ]; then
    echo -e "${BLUE}🚀 Starting Velo Test Service (Port 8082)...${NC}"
    if ! check_port 8082; then
        echo -e "${YELLOW}⚠️  Port 8082 is in use, skipping Velo Test Service${NC}"
    else
        nohup python3 services/velo_test_service.py > logs/velo_test_service.log 2>&1 &
        VELO_SERVICE_PID=$!
        echo $VELO_SERVICE_PID > logs/velo_test_service.pid
        echo -e "${GREEN}✅ Velo Test Service started (PID: $VELO_SERVICE_PID)${NC}"

        # Wait for Velo Test Service to be ready
        wait_for_service 8082 "Velo Test Service"
    fi
fi

# Save all PIDs to a master file
echo -e "${BLUE}💾 Saving service PIDs...${NC}"
cat > logs/all_services.pid << EOF
WHATSAPP_BRIDGE_PID=$WHATSAPP_PID
DROP_MONITOR_PID=$DROP_MONITOR_PID
QA_FEEDBACK_PID=$QA_FEEDBACK_PID
DONE_DETECTOR_PID=$DONE_DETECTOR_PID
VELO_SERVICE_PID=${VELO_SERVICE_PID:-}
EOF

# Final status check
echo -e "${BLUE}🔍 Performing final health check...${NC}"
sleep 5

# Check if all services are running
services_running=true

if ! kill -0 $WHATSAPP_PID 2>/dev/null; then
    echo -e "${RED}❌ WhatsApp Bridge is not running${NC}"
    services_running=false
fi

if ! kill -0 $DROP_MONITOR_PID 2>/dev/null; then
    echo -e "${RED}❌ Drop Monitor is not running${NC}"
    services_running=false
fi

if ! kill -0 $QA_FEEDBACK_PID 2>/dev/null; then
    echo -e "${RED}❌ QA Feedback Communicator is not running${NC}"
    services_running=false
fi

if ! kill -0 $DONE_DETECTOR_PID 2>/dev/null; then
    echo -e "${RED}❌ Done Message Detector is not running${NC}"
    services_running=false
fi

if [ "$services_running" = true ]; then
    echo -e "${GREEN}🎉 All Velo Test services started successfully!${NC}"
    echo -e "${GREEN}📍 Service Status:${NC}"
    echo -e "${GREEN}   - WhatsApp Bridge: http://localhost:8080 (PID: $WHATSAPP_PID)${NC}"
    echo -e "${GREEN}   - Drop Monitor: Running (PID: $DROP_MONITOR_PID)${NC}"
    echo -e "${GREEN}   - QA Feedback Communicator: Running (PID: $QA_FEEDBACK_PID)${NC}"
    echo -e "${GREEN}   - Done Message Detector: Running (PID: $DONE_DETECTOR_PID)${NC}"
    if [ ! -z "$VELO_SERVICE_PID" ]; then
        echo -e "${GREEN}   - Velo Test Service: http://localhost:8082 (PID: $VELO_SERVICE_PID)${NC}"
    fi
    echo -e "${BLUE}📋 Logs available in: logs/${NC}"
    echo -e "${BLUE}🛑 To stop all services: ./scripts/stop_all.sh${NC}"
    echo -e "${BLUE}🏥 Health check: ./scripts/health_check.sh${NC}"
else
    echo -e "${RED}❌ Some services failed to start. Check logs in logs/ directory${NC}"
    exit 1
fi