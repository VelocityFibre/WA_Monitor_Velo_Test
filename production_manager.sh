#!/bin/bash

# Production Service Manager for Velo Test WA Monitor
# Handles proper startup, shutdown, and monitoring of all services

set -e

# Configuration
PROJECT_DIR="/home/louisdup/VF/deployments/WA_monitor _Velo_Test"
SERVICES_DIR="$PROJECT_DIR/services"
BRIDGE_DIR="$SERVICES_DIR/whatsapp-bridge"
LOGS_DIR="$PROJECT_DIR/logs"
PIDS_DIR="$PROJECT_DIR/pids"

# Create necessary directories
mkdir -p "$LOGS_DIR" "$PIDS_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1"
}

success() {
    echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')] SUCCESS:${NC} $1"
}

warn() {
    echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1"
}

# Service definitions
declare -A SERVICES=(
    ["whatsapp-bridge"]="$BRIDGE_DIR/whatsapp-bridge"
    ["qa-feedback"]="python3 $SERVICES_DIR/smart_qa_feedback.py --interval 120"
    ["drop-monitor"]="python3 $SERVICES_DIR/realtime_drop_monitor.py --interval 15"
)

declare -A SERVICE_PORTS=(
    ["whatsapp-bridge"]="8080"
    ["qa-feedback"]=""
    ["drop-monitor"]=""
)

# Check if a service is running
is_service_running() {
    local service_name="$1"
    local pid_file="$PIDS_DIR/${service_name}.pid"
    
    if [[ -f "$pid_file" ]]; then
        local pid=$(cat "$pid_file")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0  # Running
        else
            # PID file exists but process is dead
            rm -f "$pid_file"
        fi
    fi
    return 1  # Not running
}

# Start a single service
start_service() {
    local service_name="$1"
    local service_cmd="${SERVICES[$service_name]}"
    local service_port="${SERVICE_PORTS[$service_name]}"
    local pid_file="$PIDS_DIR/${service_name}.pid"
    local log_file="$LOGS_DIR/${service_name}.log"
    
    if is_service_running "$service_name"; then
        warn "Service $service_name is already running (PID: $(cat $pid_file))"
        return 0
    fi
    
    # Check port availability if service uses a port
    if [[ -n "$service_port" ]]; then
        if netstat -tuln | grep -q ":$service_port "; then
            error "Port $service_port is already in use! Cannot start $service_name"
            return 1
        fi
    fi
    
    log "Starting $service_name..."
    
    # Change to appropriate directory
    local work_dir="$PROJECT_DIR"
    if [[ "$service_name" == "whatsapp-bridge" ]]; then
        work_dir="$BRIDGE_DIR"
    else
        work_dir="$SERVICES_DIR"
    fi
    
    cd "$work_dir"
    
    # Start service in background
    nohup $service_cmd > "$log_file" 2>&1 &
    local pid=$!
    
    # Save PID
    echo "$pid" > "$pid_file"
    
    # Wait a moment and check if it started successfully
    sleep 2
    if is_service_running "$service_name"; then
        success "Started $service_name (PID: $pid)"
        if [[ -n "$service_port" ]]; then
            log "Service $service_name listening on port $service_port"
        fi
    else
        error "Failed to start $service_name"
        return 1
    fi
}

# Stop a single service
stop_service() {
    local service_name="$1"
    local pid_file="$PIDS_DIR/${service_name}.pid"
    
    if ! is_service_running "$service_name"; then
        warn "Service $service_name is not running"
        return 0
    fi
    
    local pid=$(cat "$pid_file")
    log "Stopping $service_name (PID: $pid)..."
    
    # Graceful shutdown
    kill -TERM "$pid" 2>/dev/null || true
    
    # Wait up to 10 seconds for graceful shutdown
    local count=0
    while ps -p "$pid" > /dev/null 2>&1 && [[ $count -lt 10 ]]; do
        sleep 1
        count=$((count + 1))
    done
    
    # Force kill if still running
    if ps -p "$pid" > /dev/null 2>&1; then
        warn "Graceful shutdown failed, force killing $service_name"
        kill -KILL "$pid" 2>/dev/null || true
        sleep 1
    fi
    
    # Clean up PID file
    rm -f "$pid_file"
    success "Stopped $service_name"
}

# Kill all conflicting processes
cleanup_conflicts() {
    log "Cleaning up conflicting processes..."
    
    # Kill any rogue whatsapp-bridge processes
    pkill -f "whatsapp-bridge" 2>/dev/null || true
    
    # Kill any processes using port 8080
    local port_pids=$(lsof -ti:8080 2>/dev/null || true)
    if [[ -n "$port_pids" ]]; then
        warn "Killing processes using port 8080: $port_pids"
        echo "$port_pids" | xargs kill -9 2>/dev/null || true
    fi
    
    sleep 2
    success "Cleanup completed"
}

# Start all services
start_all() {
    log "Starting all Velo Test services..."
    
    # First cleanup any conflicts
    cleanup_conflicts
    
    # Start services in order (bridge first, then monitors)
    local startup_order=("whatsapp-bridge" "qa-feedback" "drop-monitor")
    
    for service in "${startup_order[@]}"; do
        start_service "$service"
        sleep 3  # Brief pause between service starts
    done
    
    success "All services started successfully!"
    show_status
}

# Stop all services
stop_all() {
    log "Stopping all Velo Test services..."
    
    for service in "${!SERVICES[@]}"; do
        stop_service "$service"
    done
    
    cleanup_conflicts
    success "All services stopped"
}

# Show service status
show_status() {
    echo
    log "=== Velo Test Service Status ==="
    
    for service in "${!SERVICES[@]}"; do
        local pid_file="$PIDS_DIR/${service}.pid"
        local log_file="$LOGS_DIR/${service}.log"
        
        if is_service_running "$service"; then
            local pid=$(cat "$pid_file")
            local port="${SERVICE_PORTS[$service]}"
            local port_info=""
            if [[ -n "$port" ]]; then
                port_info=" (Port: $port)"
            fi
            success "$service: RUNNING (PID: $pid)$port_info"
        else
            error "$service: STOPPED"
        fi
        
        # Show last few log lines
        if [[ -f "$log_file" ]]; then
            echo "  Last log: $(tail -1 "$log_file" 2>/dev/null | head -c 100)..."
        fi
        echo
    done
}

# Restart a service
restart_service() {
    local service_name="$1"
    log "Restarting $service_name..."
    stop_service "$service_name"
    sleep 2
    start_service "$service_name"
}

# Monitor services and restart if needed
monitor_services() {
    log "Starting service monitor (check every 30 seconds)..."
    
    while true; do
        for service in "${!SERVICES[@]}"; do
            if ! is_service_running "$service"; then
                warn "Service $service is down, restarting..."
                start_service "$service"
            fi
        done
        sleep 30
    done
}

# Health check
health_check() {
    log "Performing health checks..."
    
    local all_healthy=true
    
    # Check WhatsApp Bridge API
    if curl -s -f "http://localhost:8080/api/send" -X POST -H "Content-Type: application/json" -d '{"recipient":"test","message":"test"}' | grep -q "success"; then
        success "WhatsApp Bridge API: HEALTHY"
    else
        error "WhatsApp Bridge API: UNHEALTHY"
        all_healthy=false
    fi
    
    # Check log files for errors
    for service in "${!SERVICES[@]}"; do
        local log_file="$LOGS_DIR/${service}.log"
        if [[ -f "$log_file" ]]; then
            local recent_errors=$(tail -50 "$log_file" | grep -i "error\|failed" | wc -l)
            if [[ $recent_errors -gt 5 ]]; then
                warn "$service: Found $recent_errors recent errors in logs"
                all_healthy=false
            fi
        fi
    done
    
    if [[ "$all_healthy" == "true" ]]; then
        success "All health checks passed!"
        return 0
    else
        error "Some health checks failed"
        return 1
    fi
}

# Main command handling
case "${1:-help}" in
    start)
        start_all
        ;;
    stop)
        stop_all
        ;;
    restart)
        if [[ -n "$2" ]]; then
            restart_service "$2"
        else
            stop_all
            sleep 3
            start_all
        fi
        ;;
    status)
        show_status
        ;;
    monitor)
        monitor_services
        ;;
    health)
        health_check
        ;;
    cleanup)
        cleanup_conflicts
        ;;
    logs)
        if [[ -n "$2" ]]; then
            tail -f "$LOGS_DIR/${2}.log"
        else
            echo "Available logs:"
            ls -la "$LOGS_DIR/"
        fi
        ;;
    help)
        echo "Velo Test Production Manager"
        echo ""
        echo "Usage: $0 {start|stop|restart|status|monitor|health|cleanup|logs}"
        echo ""
        echo "Commands:"
        echo "  start     - Start all services"
        echo "  stop      - Stop all services" 
        echo "  restart   - Restart all services (or specific service)"
        echo "  status    - Show service status"
        echo "  monitor   - Monitor services and auto-restart if down"
        echo "  health    - Run health checks"
        echo "  cleanup   - Clean up conflicting processes"
        echo "  logs      - Show logs (specify service name)"
        echo ""
        echo "Examples:"
        echo "  $0 start                    # Start all services"
        echo "  $0 restart whatsapp-bridge  # Restart specific service"
        echo "  $0 logs qa-feedback         # Show QA feedback logs"
        ;;
    *)
        error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac