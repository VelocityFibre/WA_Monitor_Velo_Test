#!/bin/bash

# Quick End-to-End System Test
# Run this after WhatsApp authentication is complete

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🧪 QUICK END-TO-END SYSTEM TEST${NC}"
echo "=================================="
echo ""

# Test 1: Service Status
echo -e "${BLUE}📊 Test 1: Service Status${NC}"
./manage_services.sh status
echo ""

# Test 2: WhatsApp Bridge Health
echo -e "${BLUE}📱 Test 2: WhatsApp Bridge Connectivity${NC}"
if curl -s http://localhost:8080/health >/dev/null 2>&1; then
    echo -e "${GREEN}✅ WhatsApp Bridge health check passed${NC}"
else
    echo -e "${RED}❌ WhatsApp Bridge health check failed${NC}"
fi
echo ""

# Test 3: Database Connection
echo -e "${BLUE}💾 Test 3: Database Connectivity${NC}"
if source .venv/bin/activate 2>/dev/null && python3 -c "
import psycopg2
import os
try:
    conn = psycopg2.connect(os.environ['NEON_DATABASE_URL'])
    print('✅ Database connection successful')
    cursor = conn.cursor()
    cursor.execute('SELECT 1;')
    result = cursor.fetchone()
    print(f'✅ Database query test: {result[0]}')
    conn.close()
except Exception as e:
    print(f'❌ Database test failed: {e}')
" 2>/dev/null; then
    echo -e "${GREEN}✅ Database tests passed${NC}"
else
    echo -e "${RED}❌ Database tests failed${NC}"
fi
echo ""

# Test 4: Google Sheets API
echo -e "${BLUE}📊 Test 4: Google Sheets Integration${NC}"
if source .venv/bin/activate 2>/dev/null && python3 -c "
from google.oauth2.service_account import Credentials
from googleapiclient.discovery import build
import os
try:
    creds = Credentials.from_service_account_file(os.environ['GOOGLE_SHEETS_CREDENTIALS_PATH'])
    service = build('sheets', 'v4', credentials=creds)
    sheet = service.spreadsheets()
    result = sheet.values().get(
        spreadsheetId=os.environ['GOOGLE_SHEETS_ID'], 
        range='Velo Test!A1:C1'
    ).execute()
    print('✅ Google Sheets API connection successful')
    print(f'✅ Sheet access test: {len(result.get(\"values\", []))} rows read')
except Exception as e:
    print(f'❌ Google Sheets test failed: {e}')
" 2>/dev/null; then
    echo -e "${GREEN}✅ Google Sheets tests passed${NC}"
else
    echo -e "${RED}❌ Google Sheets tests failed${NC}"
fi
echo ""

# Test 5: Recent Service Activity
echo -e "${BLUE}📋 Test 5: Recent Service Activity${NC}"
echo "Drop Monitor activity:"
tail -2 logs/drop-monitor.log 2>/dev/null | head -1 || echo "No recent activity"
echo "Done Detector activity:"
tail -2 logs/done-detector.log 2>/dev/null | head -1 || echo "No recent activity"
echo ""

# Test 6: System Resources
echo -e "${BLUE}💻 Test 6: System Resources${NC}"
echo "Memory usage: $(free -h | grep '^Mem' | awk '{print $3 "/" $2}')"
echo "CPU usage: $(top -bn1 | grep '^%Cpu' | awk '{print $2}' | sed 's/%us,//')"
echo ""

# Summary
echo -e "${BLUE}📋 SYSTEM TEST SUMMARY${NC}"
echo "======================="
echo ""
echo -e "${GREEN}✅ Ready for end-to-end workflow testing!${NC}"
echo ""
echo -e "${YELLOW}📱 Next Steps:${NC}"
echo "1. Ensure WhatsApp is authenticated (no QR code in logs)"
echo "2. Test drop detection: Post 'Testing DR9999999' in Velo Test WhatsApp group"
echo "3. Check database and Google Sheets for new entry"
echo "4. Test QA feedback workflow"
echo ""
echo -e "${BLUE}🔍 Monitor real-time activity:${NC}"
echo "   tail -f logs/drop-monitor.log"
echo "   tail -f logs/whatsapp-bridge.log"