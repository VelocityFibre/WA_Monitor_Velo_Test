#!/bin/bash

# Simple Service Management Script for WA Monitor
# Usage: ./manage_services.sh [start|stop|restart|status]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project directory
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_DIR"

# Service definitions
declare -A SERVICES=(
    ["whatsapp-bridge"]="cd services/whatsapp-bridge && go run main.go"
    ["drop-monitor"]="python3 services/realtime_drop_monitor.py"
    ["qa-feedback"]="python3 services/qa_feedback_communicator.py" 
    ["done-detector"]="python3 services/done_message_detector.py"
)

# Create logs directory
mkdir -p logs

# Function to start a service
start_service() {
    local service_name=$1
    local command="${SERVICES[$service_name]}"
    
    if [ -z "$command" ]; then
        echo -e "${RED}❌ Unknown service: $service_name${NC}"
        return 1
    fi
    
    # Check if already running
    if pgrep -f "$service_name" > /dev/null 2>&1; then
        echo -e "${YELLOW}⚠️  Service $service_name is already running${NC}"
        return 0
    fi
    
    echo -e "${BLUE}🚀 Starting $service_name...${NC}"
    
    # Activate Python virtual environment if it exists
    if [ -d ".venv" ]; then
        source .venv/bin/activate
    fi
    
    # Export environment variables from .env file
    if [ -f ".env" ]; then
        set -a  # automatically export all variables
        source .env
        set +a  # stop auto-export
    fi
    
    # Start service in background
    nohup bash -c "$command" > "logs/${service_name}.log" 2>&1 &
    local pid=$!
    echo $pid > "logs/${service_name}.pid"
    
    # Wait a moment and check if it started
    sleep 2
    if kill -0 $pid 2>/dev/null; then
        echo -e "${GREEN}✅ $service_name started successfully (PID: $pid)${NC}"
    else
        echo -e "${RED}❌ Failed to start $service_name${NC}"
        return 1
    fi
}

# Function to stop a service
stop_service() {
    local service_name=$1
    local pid_file="logs/${service_name}.pid"
    
    echo -e "${YELLOW}🛑 Stopping $service_name...${NC}"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            sleep 2
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid" 2>/dev/null || true
            fi
            rm -f "$pid_file"
            echo -e "${GREEN}✅ $service_name stopped${NC}"
        else
            echo -e "${YELLOW}⚠️  $service_name was not running${NC}"
            rm -f "$pid_file"
        fi
    else
        # Try to kill by process name
        pkill -f "$service_name" 2>/dev/null || echo -e "${YELLOW}⚠️  No $service_name process found${NC}"
    fi
}

# Function to check service status
service_status() {
    local service_name=$1
    local pid_file="logs/${service_name}.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            echo -e "${GREEN}✅ $service_name is running (PID: $pid)${NC}"
            return 0
        else
            echo -e "${RED}❌ $service_name is not running (stale PID file)${NC}"
            rm -f "$pid_file"
            return 1
        fi
    else
        echo -e "${RED}❌ $service_name is not running${NC}"
        return 1
    fi
}

# Function to start all services
start_all() {
    echo -e "${BLUE}🚀 Starting all services...${NC}"
    echo ""
    
    # Start services in order
    start_service "whatsapp-bridge"
    sleep 3
    start_service "drop-monitor"
    start_service "qa-feedback"
    start_service "done-detector"
    
    echo ""
    echo -e "${GREEN}🎉 All services startup attempted${NC}"
    echo ""
    status_all
}

# Function to stop all services
stop_all() {
    echo -e "${YELLOW}🛑 Stopping all services...${NC}"
    echo ""
    
    for service in "${!SERVICES[@]}"; do
        stop_service "$service"
    done
    
    echo ""
    echo -e "${GREEN}✅ All services stopped${NC}"
}

# Function to show status of all services
status_all() {
    echo -e "${BLUE}📊 Service Status:${NC}"
    echo ""
    
    local all_running=true
    for service in "${!SERVICES[@]}"; do
        if ! service_status "$service"; then
            all_running=false
        fi
    done
    
    echo ""
    if $all_running; then
        echo -e "${GREEN}🎉 All services are running!${NC}"
    else
        echo -e "${YELLOW}⚠️  Some services are not running${NC}"
    fi
    
    # Show ports
    echo ""
    echo -e "${BLUE}🌐 Port Status:${NC}"
    lsof -i :8080 2>/dev/null | grep LISTEN || echo "Port 8080: Available"
    lsof -i :8082 2>/dev/null | grep LISTEN || echo "Port 8082: Available"
}

# Function to restart all services
restart_all() {
    echo -e "${YELLOW}🔄 Restarting all services...${NC}"
    stop_all
    sleep 3
    start_all
}

# Function to show logs
show_logs() {
    local service_name=${1:-"all"}
    
    if [ "$service_name" = "all" ]; then
        echo -e "${BLUE}📋 Recent logs from all services:${NC}"
        echo ""
        for service in "${!SERVICES[@]}"; do
            if [ -f "logs/${service}.log" ]; then
                echo -e "${YELLOW}=== $service ===${NC}"
                tail -3 "logs/${service}.log" 2>/dev/null || echo "No recent logs"
                echo ""
            fi
        done
    else
        if [ -f "logs/${service_name}.log" ]; then
            echo -e "${BLUE}📋 Logs for $service_name:${NC}"
            tail -20 "logs/${service_name}.log"
        else
            echo -e "${RED}❌ No log file for $service_name${NC}"
        fi
    fi
}

# Function to run health checks
health_check() {
    echo -e "${BLUE}🏥 Running health checks...${NC}"
    echo ""
    
    # Check WhatsApp Bridge
    if curl -s http://localhost:8080/health >/dev/null 2>&1; then
        echo -e "${GREEN}✅ WhatsApp Bridge health check passed${NC}"
    else
        echo -e "${RED}❌ WhatsApp Bridge health check failed${NC}"
    fi
    
    # Check database connection
    if source .venv/bin/activate 2>/dev/null && python3 -c "
import psycopg2
import os
try:
    conn = psycopg2.connect(os.environ['NEON_DATABASE_URL'])
    print('✅ Database connection successful')
    conn.close()
except Exception as e:
    print(f'❌ Database connection failed: {e}')
" 2>/dev/null; then
        echo -e "${GREEN}✅ Database health check passed${NC}"
    else
        echo -e "${RED}❌ Database health check failed${NC}"
    fi
}

# Main script logic
case "${1:-status}" in
    "start")
        if [ -n "$2" ]; then
            start_service "$2"
        else
            start_all
        fi
        ;;
    "stop")
        if [ -n "$2" ]; then
            stop_service "$2"
        else
            stop_all
        fi
        ;;
    "restart")
        if [ -n "$2" ]; then
            stop_service "$2"
            sleep 2
            start_service "$2"
        else
            restart_all
        fi
        ;;
    "status")
        status_all
        ;;
    "logs")
        show_logs "$2"
        ;;
    "health")
        health_check
        ;;
    *)
        echo "Usage: $0 [start|stop|restart|status|logs|health] [service_name]"
        echo ""
        echo "Available services:"
        for service in "${!SERVICES[@]}"; do
            echo "  - $service"
        done
        echo ""
        echo "Examples:"
        echo "  $0 start                    # Start all services"
        echo "  $0 stop whatsapp-bridge     # Stop specific service"
        echo "  $0 logs drop-monitor        # Show logs for service"
        echo "  $0 health                   # Run health checks"
        exit 1
        ;;
esac