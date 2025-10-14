#!/bin/bash

# Velo Test - Health Check Script
# This script checks the health of all Velo Test services

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

echo -e "${BLUE}🏥 Velo Test Service Health Check${NC}"
echo -e "${BLUE}Project Directory: $PROJECT_DIR${NC}"
echo ""

# Load environment variables if .env exists
if [ -f ".env" ]; then
    source .env
else
    echo -e "${YELLOW}⚠️  .env file not found, some checks may be limited${NC}"
fi

# Initialize counters
total_checks=0
passed_checks=0
failed_checks=0
warning_checks=0

# Function to check service
check_service() {
    local service_name=$1
    local check_command=$2
    local expected_result=${3:-"0"}
    local port=${4:-""}
    local description=$5

    ((total_checks++))

    echo -e "${BLUE}🔍 Checking: $service_name${NC}"
    if [ -n "$description" ]; then
        echo -e "   $description"
    fi

    # Run the check command
    if eval "$check_command" >/dev/null 2>&1; then
        local result=$?
        if [ "$result" -eq "$expected_result" ]; then
            echo -e "   ${GREEN}✅ PASS${NC}"
            if [ -n "$port" ]; then
                echo -e "   ${GREEN}   Accessible on port $port${NC}"
            fi
            ((passed_checks++))
            return 0
        else
            echo -e "   ${RED}❌ FAIL (Exit code: $result)${NC}"
            ((failed_checks++))
            return 1
        fi
    else
        echo -e "   ${RED}❌ FAIL (Command failed)${NC}"
        ((failed_checks++))
        return 1
    fi
}

# Function to check file/directory exists
check_exists() {
    local name=$1
    local path=$2
    local type=${3:-"file"}

    ((total_checks++))
    echo -e "${BLUE}🔍 Checking: $name${NC}"

    if [ "$type" = "directory" ]; then
        if [ -d "$path" ]; then
            echo -e "   ${GREEN}✅ PASS ($path)${NC}"
            ((passed_checks++))
            return 0
        else
            echo -e "   ${RED}❌ FAIL ($path not found)${NC}"
            ((failed_checks++))
            return 1
        fi
    else
        if [ -f "$path" ]; then
            echo -e "   ${GREEN}✅ PASS ($path)${NC}"
            ((passed_checks++))
            return 0
        else
            echo -e "   ${RED}❌ FAIL ($path not found)${NC}"
            ((failed_checks++))
            return 1
        fi
    fi
}

# Function to check environment variable
check_env_var() {
    local var_name=$1
    local required=${2:-"true"}

    ((total_checks++))
    echo -e "${BLUE}🔍 Checking: Environment variable $var_name${NC}"

    if [ -z "${!var_name}" ]; then
        if [ "$required" = "true" ]; then
            echo -e "   ${RED}❌ FAIL (Not set)${NC}"
            ((failed_checks++))
            return 1
        else
            echo -e "   ${YELLOW}⚠️  WARNING (Not set, but optional)${NC}"
            ((warning_checks++))
            return 0
        fi
    else
        echo -e "   ${GREEN}✅ PASS (Set)${NC}"
        if [ "$var_name" = "NEON_DATABASE_URL" ] || [ "$var_name" = "LLM_API_KEY" ]; then
            echo -e "   ${GREEN}   Value: ${!var_name:0:20}...${NC}"
        else
            echo -e "   ${GREEN}   Value: ${!var_name}${NC}"
        fi
        ((passed_checks++))
        return 0
    fi
}

echo -e "${YELLOW}📋 Basic Infrastructure Checks${NC}"
echo ""

# Check essential directories
check_exists "Services directory" "services" "directory"
check_exists "Scripts directory" "scripts" "directory"
check_exists "Logs directory" "logs" "directory"
check_exists "Docker data directory" "docker-data" "directory"

# Check essential files
check_exists "Requirements file" "services/requirements.txt"
check_exists "Environment template" ".env.template"
check_exists "Drop monitor script" "services/realtime_drop_monitor.py"
check_exists "QA feedback script" "services/qa_feedback_communicator.py"
check_exists "Done detector script" "services/done_message_detector.py"

echo ""
echo -e "${YELLOW}🐍 Python Environment Checks${NC}"
echo ""

# Check Python installation
if command -v python3 >/dev/null 2>&1; then
    python_version=$(python3 --version 2>&1)
    echo -e "${GREEN}✅ Python3 installed: $python_version${NC}"
    ((passed_checks++))
else
    echo -e "${RED}❌ Python3 not found${NC}"
    ((failed_checks++))
fi
((total_checks++))

# Check virtual environment
if [ -d ".venv" ]; then
    echo -e "${GREEN}✅ Virtual environment exists${NC}"
    ((passed_checks++))

    # Check if virtual environment is activated
    if [ "$VIRTUAL_ENV" != "" ]; then
        echo -e "${GREEN}✅ Virtual environment is activated${NC}"
        ((passed_checks++))
        ((total_checks++))
    else
        echo -e "${YELLOW}⚠️  Virtual environment exists but not activated${NC}"
        ((warning_checks++))
        ((total_checks++))
    fi
else
    echo -e "${YELLOW}⚠️  Virtual environment not found (run: python3 -m venv .venv)${NC}"
    ((warning_checks++))
fi
((total_checks++))

echo ""
echo -e "${YELLOW}🐹 Go Environment Checks${NC}"
echo ""

# Check Go installation
if command -v go >/dev/null 2>&1; then
    go_version=$(go version 2>&1)
    echo -e "${GREEN}✅ Go installed: $go_version${NC}"
    ((passed_checks++))
else
    echo -e "${RED}❌ Go not found${NC}"
    ((failed_checks++))
fi
((total_checks++))

# Check Go module files
if [ -f "services/whatsapp-bridge/go.mod" ]; then
    echo -e "${GREEN}✅ Go module file exists${NC}"
    ((passed_checks++))
else
    echo -e "${RED}❌ Go module file not found${NC}"
    ((failed_checks++))
fi
((total_checks++))

echo ""
echo -e "${YELLOW}🔗 Service Connectivity Checks${NC}"
echo ""

# Check WhatsApp Bridge (Port 8080)
check_service "WhatsApp Bridge" "curl -s --max-time 5 http://localhost:8080/health" "0" "8080" "WhatsApp Web integration service"

# Check Velo Test Service (Port 8082) - if available
check_service "Velo Test Service" "curl -s --max-time 5 http://localhost:8082/health" "0" "8082" "Velo Test monitoring service" || true

# Check if processes are running by PID files
echo ""
echo -e "${YELLOW}📋 Process Status Checks${NC}"
echo ""

check_service "WhatsApp Bridge Process" "kill -0 \$(cat logs/whatsapp_bridge.pid 2>/dev/null) 2>/dev/null" "0" "" "Running Go application"

check_service "Drop Monitor Process" "kill -0 \$(cat logs/drop_monitor.pid 2>/dev/null) 2>/dev/null" "0" "" "Python drop detection service"

check_service "QA Feedback Process" "kill -0 \$(cat logs/qa_feedback.pid 2>/dev/null) 2>/dev/null" "0" "" "Python QA feedback service"

check_service "Done Detector Process" "kill -0 \$(cat logs/done_detector.pid 2>/dev/null) 2>/dev/null" "0" "" "Python resubmission handler"

# Fallback process checks using pgrep
echo ""
echo -e "${YELLOW}🔄 Fallback Process Checks${NC}"
echo ""

check_service "Drop Monitor (pgrep)" "pgrep -f realtime_drop_monitor.py" "" "" "Python drop detection process"

check_service "QA Feedback (pgrep)" "pgrep -f qa_feedback_communicator.py" "" "" "Python QA feedback process"

check_service "Done Detector (pgrep)" "pgrep -f done_message_detector.py" "" "" "Python done detection process"

check_service "WhatsApp Bridge (pgrep)" "pgrep -f 'go run main.go'" "" "" "Go WhatsApp bridge process"

echo ""
echo -e "${YELLOW}🌐 External Service Checks${NC}"
echo ""

# Check environment variables
check_env_var "NEON_DATABASE_URL"
check_env_var "GOOGLE_SHEETS_CREDENTIALS_PATH"
check_env_var "GOOGLE_SHEETS_ID"
check_env_var "LLM_API_KEY"
check_env_var "VELO_TEST_GROUP_JID"

# Test database connection if URL is available
if [ ! -z "$NEON_DATABASE_URL" ]; then
    echo -e "${BLUE}🔍 Checking: Neon Database Connection${NC}"
    ((total_checks++))
    if python3 -c "
import psycopg2
import sys
try:
    conn = psycopg2.connect('$NEON_DATABASE_URL')
    conn.close()
    print('✅ Database connection successful')
    sys.exit(0)
except Exception as e:
    print(f'❌ Database connection failed: {e}')
    sys.exit(1)
" 2>/dev/null; then
        echo -e "   ${GREEN}✅ PASS${NC}"
        ((passed_checks++))
    else
        echo -e "   ${RED}❌ FAIL${NC}"
        ((failed_checks++))
    fi
fi

# Test Google Sheets API if credentials are available
if [ ! -z "$GOOGLE_SHEETS_CREDENTIALS_PATH" ] && [ -f "$GOOGLE_SHEETS_CREDENTIALS_PATH" ]; then
    echo -e "${BLUE}🔍 Checking: Google Sheets API Access${NC}"
    ((total_checks++))
    if python3 -c "
try:
    from google.oauth2.service_account import Credentials
    from googleapiclient.discovery import build
    creds = Credentials.from_service_account_file('$GOOGLE_SHEETS_CREDENTIALS_PATH', scopes=['https://www.googleapis.com/auth/spreadsheets'])
    service = build('sheets', 'v4', credentials=creds)
    # Test API access by trying to get spreadsheet metadata
    service.spreadsheets().get(spreadsheetId='$GOOGLE_SHEETS_ID').execute()
    print('✅ Google Sheets API accessible')
except Exception as e:
    print(f'❌ Google Sheets API failed: {e}')
    exit(1)
" 2>/dev/null; then
        echo -e "   ${GREEN}✅ PASS${NC}"
        ((passed_checks++))
    else
        echo -e "   ${RED}❌ FAIL${NC}"
        ((failed_checks++))
    fi
fi

# Test OpenRouter API if key is available
if [ ! -z "$LLM_API_KEY" ]; then
    echo -e "${BLUE}🔍 Checking: OpenRouter API Access${NC}"
    ((total_checks++))
    if curl -s --max-time 10 -H "Authorization: Bearer $LLM_API_KEY" "https://openrouter.ai/api/v1/models" >/dev/null 2>&1; then
        echo -e "   ${GREEN}✅ PASS${NC}"
        ((passed_checks++))
    else
        echo -e "   ${RED}❌ FAIL${NC}"
        ((failed_checks++))
    fi
fi

echo ""
echo -e "${YELLOW}📊 Health Check Summary${NC}"
echo ""

# Calculate success rate
if [ $total_checks -gt 0 ]; then
    success_rate=$(( (passed_checks * 100) / total_checks ))
else
    success_rate=0
fi

echo -e "${BLUE}Total Checks: $total_checks${NC}"
echo -e "${GREEN}Passed: $passed_checks${NC}"
echo -e "${YELLOW}Warnings: $warning_checks${NC}"
echo -e "${RED}Failed: $failed_checks${NC}"
echo ""
echo -e "${BLUE}Success Rate: $success_rate%${NC}"

# Overall status determination
if [ $failed_checks -eq 0 ]; then
    if [ $warning_checks -eq 0 ]; then
        echo -e ""
        echo -e "${GREEN}🎉 Overall System Status: HEALTHY${NC}"
        echo -e "${GREEN}   All services are running correctly!${NC}"
        exit_code=0
    else
        echo -e ""
        echo -e "${YELLOW}⚠️  Overall System Status: HEALTHY (with warnings)${NC}"
        echo -e "${YELLOW}   Services are running, but some optional checks failed${NC}"
        exit_code=0
    fi
else
    if [ $success_rate -ge 70 ]; then
        echo -e ""
        echo -e "${YELLOW}⚠️  Overall System Status: DEGRADED${NC}"
        echo -e "${YELLOW}   Some services may not be functioning correctly${NC}"
        exit_code=1
    else
        echo -e ""
        echo -e "${RED}❌ Overall System Status: UNHEALTHY${NC}"
        echo -e "${RED}   Multiple services are not working correctly${NC}"
        exit_code=2
    fi
fi

echo ""
echo -e "${BLUE}📋 Quick Actions:${NC}"
echo -e "${BLUE}   • Start all services: ./scripts/start_all.sh${NC}"
echo -e "${BLUE}   • Stop all services: ./scripts/stop_all.sh${NC}"
echo -e "${BLUE}   • View logs: tail -f logs/service_name.log${NC}"
echo -e "${BLUE}   • Restart services: ./scripts/stop_all.sh && ./scripts/start_all.sh${NC}"

exit $exit_code