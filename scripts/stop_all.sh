#!/bin/bash

# Velo Test - Stop All Services
# This script gracefully stops all Velo Test services

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

echo -e "${BLUE}🛑 Stopping Velo Test Services...${NC}"
echo -e "${BLUE}Project Directory: $PROJECT_DIR${NC}"

# Function to stop service by PID file
stop_service() {
    local pid_file=$1
    local service_name=$2

    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            echo -e "${YELLOW}⏹️  Stopping $service_name (PID: $pid)...${NC}"
            kill -TERM "$pid"

            # Wait for graceful shutdown
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
                sleep 1
                ((count++))
            done

            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                echo -e "${YELLOW}⚠️  Force killing $service_name...${NC}"
                kill -KILL "$pid" 2>/dev/null || true
            fi

            echo -e "${GREEN}✅ $service_name stopped${NC}"
        else
            echo -e "${YELLOW}⚠️  $service_name PID $pid not running${NC}"
        fi
        rm -f "$pid_file"
    else
        echo -e "${YELLOW}⚠️  $service_name PID file not found${NC}"
    fi
}

# Function to stop service by name
stop_service_by_name() {
    local service_name=$1
    local process_pattern=$2

    echo -e "${YELLOW}🔍 Looking for $service_name processes...${NC}"

    local pids=$(pgrep -f "$process_pattern" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        echo -e "${YELLOW}⏹️  Stopping $service_name processes: $pids${NC}"
        echo "$pids" | xargs kill -TERM 2>/dev/null || true

        # Wait for graceful shutdown
        sleep 3

        # Force kill if still running
        local remaining_pids=$(pgrep -f "$process_pattern" 2>/dev/null || true)
        if [ -n "$remaining_pids" ]; then
            echo -e "${YELLOW}⚠️  Force killing remaining $service_name processes: $remaining_pids${NC}"
            echo "$remaining_pids" | xargs kill -KILL 2>/dev/null || true
        fi

        echo -e "${GREEN}✅ $service_name processes stopped${NC}"
    else
        echo -e "${YELLOW}⚠️  No $service_name processes found${NC}"
    fi
}

# Stop services using PID files (preferred method)
echo -e "${BLUE}📋 Stopping services using PID files...${NC}"

stop_service "logs/whatsapp_bridge.pid" "WhatsApp Bridge"
stop_service "logs/drop_monitor.pid" "Drop Monitor"
stop_service "logs/qa_feedback.pid" "QA Feedback Communicator"
stop_service "logs/done_detector.pid" "Done Message Detector"
stop_service "logs/velo_test_service.pid" "Velo Test Service"

# Stop master PID file
if [ -f "logs/all_services.pid" ]; then
    rm -f "logs/all_services.pid"
fi

# Fallback: Stop services by process name
echo -e "${BLUE}🔄 Fallback: Checking for any remaining processes...${NC}"

stop_service_by_name "WhatsApp Bridge" "go run main.go"
stop_service_by_name "Drop Monitor" "realtime_drop_monitor.py"
stop_service_by_name "QA Feedback Communicator" "qa_feedback_communicator.py"
stop_service_by_name "Done Message Detector" "done_message_detector.py"
stop_service_by_name "Velo Test Service" "velo_test_service.py"

# Additional cleanup for any Go processes on port 8080
echo -e "${BLUE}🔍 Cleaning up any processes on port 8080...${NC}"
local port_8080_pid=$(lsof -ti:8080 2>/dev/null || true)
if [ -n "$port_8080_pid" ]; then
    echo -e "${YELLOW}⏹️  Stopping process on port 8080 (PID: $port_8080_pid)${NC}"
    kill -TERM "$port_8080_pid" 2>/dev/null || true
    sleep 2
    if kill -0 "$port_8080_pid" 2>/dev/null; then
        kill -KILL "$port_8080_pid" 2>/dev/null || true
    fi
fi

# Additional cleanup for any processes on port 8082
echo -e "${BLUE}🔍 Cleaning up any processes on port 8082...${NC}"
local port_8082_pid=$(lsof -ti:8082 2>/dev/null || true)
if [ -n "$port_8082_pid" ]; then
    echo -e "${YELLOW}⏹️  Stopping process on port 8082 (PID: $port_8082_pid)${NC}"
    kill -TERM "$port_8082_pid" 2>/dev/null || true
    sleep 2
    if kill -0 "$port_8082_pid" 2>/dev/null; then
        kill -KILL "$port_8082_pid" 2>/dev/null || true
    fi
fi

# Clean up any remaining Python processes related to our services
echo -e "${BLUE}🐍 Cleaning up Python service processes...${NC}"
python_pids=$(pgrep -f "realtime_drop_monitor|qa_feedback_communicator|done_message_detector|velo_test_service" 2>/dev/null || true)
if [ -n "$python_pids" ]; then
    echo -e "${YELLOW}⏹️  Stopping Python service processes: $python_pids${NC}"
    echo "$python_pids" | xargs kill -TERM 2>/dev/null || true
    sleep 2

    # Force kill if still running
    remaining_python_pids=$(pgrep -f "realtime_drop_monitor|qa_feedback_communicator|done_message_detector|velo_test_service" 2>/dev/null || true)
    if [ -n "$remaining_python_pids" ]; then
        echo -e "${YELLOW}⚠️  Force killing remaining Python processes: $remaining_python_pids${NC}"
        echo "$remaining_python_pids" | xargs kill -KILL 2>/dev/null || true
    fi
fi

# Final verification
echo -e "${BLUE}🔍 Final verification - checking for any remaining processes...${NC}"

# Check for any remaining processes
remaining_processes=""

# Check Go processes
go_processes=$(pgrep -f "go run main.go" 2>/dev/null || true)
if [ -n "$go_processes" ]; then
    remaining_processes="$remaining_processes Go:$go_processes"
fi

# Check Python processes
python_processes=$(pgrep -f "realtime_drop_monitor|qa_feedback_communicator|done_message_detector|velo_test_service" 2>/dev/null || true)
if [ -n "$python_processes" ]; then
    remaining_processes="$remaining_processes Python:$python_processes"
fi

# Check port usage
port_8080_check=$(lsof -ti:8080 2>/dev/null || true)
port_8082_check=$(lsof -ti:8082 2>/dev/null || true)
if [ -n "$port_8080_check" ] || [ -n "$port_8082_check" ]; then
    remaining_processes="$remaining_processes Ports:8080=$port_8080_check,8082=$port_8082_check"
fi

if [ -n "$remaining_processes" ]; then
    echo -e "${YELLOW}⚠️  Some processes may still be running: $remaining_processes${NC}"
    echo -e "${YELLOW}   You may need to manually kill them or reboot${NC}"
else
    echo -e "${GREEN}✅ All Velo Test services stopped successfully${NC}"
fi

# Clean up log files if requested
if [ "$1" = "--clean-logs" ]; then
    echo -e "${BLUE}🗑️  Cleaning up log files...${NC}"
    rm -f logs/*.pid
    echo -e "${GREEN}✅ Log files cleaned${NC}"
fi

echo -e "${GREEN}🎉 Velo Test services shutdown complete!${NC}"
echo -e "${BLUE}📋 To restart services: ./scripts/start_all.sh${NC}"
echo -e "${BLUE}🏥 Check service status: ./scripts/health_check.sh${NC}"