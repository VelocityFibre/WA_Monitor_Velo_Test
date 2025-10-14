# Local End-to-End Testing Plan

**Goal**: Validate complete workflow functionality before cloud deployment

**Date**: 2025-10-14  
**Phase**: 2B - Complete Local Testing  
**Status**: 🔄 IN PROGRESS

---

## 📋 Testing Workflow Overview

### Complete End-to-End Flow:
1. **WhatsApp Authentication** → Agent connects to monitored groups
2. **Drop Detection** → Agent posts "DR8888888" in Velo Test group  
3. **Database Sync** → Drop number stored in Neon PostgreSQL
4. **Google Sheets Integration** → New row added automatically
5. **QA Review** → QA team marks "Incomplete = TRUE" in sheets
6. **AI Feedback Generation** → System generates contextual feedback
7. **WhatsApp Notification** → Automated message sent to agent
8. **Resubmission** → Agent posts "DR8888888 DONE"
9. **Completion Update** → Status updated in database and sheets

---

## 🧪 Test Cases

### Test Case 1: WhatsApp Authentication & Connection
**Objective**: Ensure WhatsApp Bridge connects and maintains session

**Steps**:
1. [ ] Start WhatsApp Bridge service
2. [ ] Verify QR code authentication (or persistent session)
3. [ ] Confirm connection to Velo Test group (120363421664266245@g.us)
4. [ ] Test health endpoint: `curl http://localhost:8080/health`

**Expected Results**:
- [ ] Bridge service running on port 8080
- [ ] WhatsApp authenticated without QR scan (persistent session)
- [ ] Health endpoint returns success
- [ ] Group connection established

### Test Case 2: Drop Number Detection
**Objective**: Test real-time drop number detection from WhatsApp

**Steps**:
1. [ ] Post test message: "Testing DR9999999 drop detection"
2. [ ] Monitor Drop Monitor logs for detection
3. [ ] Verify database entry created
4. [ ] Check Google Sheets for new row

**Expected Results**:
- [ ] Drop number DR9999999 detected within 15 seconds
- [ ] Database record created in PostgreSQL
- [ ] Google Sheets row added automatically
- [ ] Logs show successful processing

### Test Case 3: Google Sheets Integration
**Objective**: Validate bidirectional Google Sheets synchronization

**Steps**:
1. [ ] Verify new drop appears in Velo Test sheet
2. [ ] Manually set "Incomplete = TRUE" in column V
3. [ ] Wait for QA Feedback Communicator to detect change
4. [ ] Verify feedback triggers correctly

**Expected Results**:
- [ ] Drop data appears in correct sheet columns
- [ ] Manual sheet changes detected by services
- [ ] QA status changes trigger feedback workflow
- [ ] Sheet permissions working correctly

### Test Case 4: AI-Powered QA Feedback
**Objective**: Test automated feedback generation and delivery

**Steps**:
1. [ ] Mark test drop as incomplete in Google Sheets
2. [ ] Monitor QA Feedback Communicator logs
3. [ ] Verify OpenRouter AI API call
4. [ ] Check WhatsApp group for feedback message

**Expected Results**:
- [ ] AI generates contextual feedback message
- [ ] Message sent to correct WhatsApp group
- [ ] Feedback includes specific missing steps
- [ ] Cooldown period respected (300 seconds)

### Test Case 5: Resubmission Detection
**Objective**: Test completion workflow when agent resubmits

**Steps**:
1. [ ] Post resubmission message: "DR9999999 DONE"
2. [ ] Monitor Done Message Detector logs
3. [ ] Verify database status update
4. [ ] Check Google Sheets completion status

**Expected Results**:
- [ ] Resubmission detected within 30 seconds
- [ ] Database "completed" flag set to TRUE
- [ ] Google Sheets "Resubmitted" column updated
- [ ] Process completion logged

### Test Case 6: Emergency Kill Switch
**Objective**: Validate emergency stop functionality

**Steps**:
1. [ ] Post emergency message: "KILL" in monitored group
2. [ ] Verify all services stop gracefully
3. [ ] Test restart capability

**Expected Results**:
- [ ] All monitoring services stop immediately
- [ ] Confirmation message sent to group
- [ ] Services can be restarted cleanly

### Test Case 7: Database Connectivity & Performance
**Objective**: Stress test database operations

**Steps**:
1. [ ] Test multiple concurrent drops
2. [ ] Verify database connection pooling
3. [ ] Check query performance
4. [ ] Validate data integrity

**Expected Results**:
- [ ] No connection failures under load
- [ ] Response times < 5 seconds
- [ ] All data correctly stored
- [ ] No duplicate entries

### Test Case 8: Service Recovery & Persistence
**Objective**: Test system resilience

**Steps**:
1. [ ] Stop individual services
2. [ ] Restart services
3. [ ] Verify session persistence
4. [ ] Test data recovery

**Expected Results**:
- [ ] Services restart without manual intervention
- [ ] WhatsApp session persists across restarts
- [ ] No data loss during restarts
- [ ] Services resume processing correctly

---

## 🔧 Pre-Testing Checklist

- [ ] All services running and healthy
- [ ] WhatsApp session authenticated
- [ ] Google Sheets credentials valid
- [ ] Database connection confirmed
- [ ] OpenRouter API key active
- [ ] Test environment variables set
- [ ] Backup of current session data

## 🛠️ Testing Commands

### Service Status Check
```bash
# Check all running services
ps aux | grep -E "(go run main.go|python3.*services)" | grep -v grep

# Verify ports
lsof -i :8080  # WhatsApp Bridge
lsof -i :8082  # Management service (optional)
```

### Health Checks
```bash
# WhatsApp Bridge health
curl http://localhost:8080/health

# Full system health
./scripts/health_check.sh

# Service logs
tail -f logs/whatsapp_bridge.log
tail -f logs/drop_monitor.log  
tail -f logs/qa_feedback.log
tail -f logs/done_detector.log
```

### Database Testing
```bash
# Test database connection
source .venv/bin/activate && python3 -c "
import psycopg2
import os
conn = psycopg2.connect(os.environ['NEON_DATABASE_URL'])
print('✅ Database connected')
conn.close()
"

# Check recent drops
source .venv/bin/activate && python3 -c "
import psycopg2
import os
conn = psycopg2.connect(os.environ['NEON_DATABASE_URL'])
cur = conn.cursor()
cur.execute('SELECT drop_number, created_at FROM drops ORDER BY created_at DESC LIMIT 5;')
print('Recent drops:', cur.fetchall())
conn.close()
"
```

### Google Sheets Testing
```bash
# Test Google Sheets API
source .venv/bin/activate && python3 -c "
from google.oauth2.service_account import Credentials
from googleapiclient.discovery import build
import os
creds = Credentials.from_service_account_file(os.environ['GOOGLE_SHEETS_CREDENTIALS_PATH'])
service = build('sheets', 'v4', credentials=creds)
sheet = service.spreadsheets()
result = sheet.values().get(spreadsheetId=os.environ['GOOGLE_SHEETS_ID'], range='Velo Test!A1:C1').execute()
print('✅ Google Sheets API working:', result.get('values', []))
"
```

---

## 📊 Success Criteria

### Functional Requirements
- [ ] Drop detection: <15 seconds response time
- [ ] QA feedback: <30 seconds generation and delivery
- [ ] Resubmission: <30 seconds processing
- [ ] Database operations: <5 seconds query time
- [ ] WhatsApp messages: Delivered successfully
- [ ] System uptime: No crashes during 2-hour test

### Integration Requirements  
- [ ] WhatsApp ↔ Database sync working
- [ ] Database ↔ Google Sheets sync working
- [ ] AI feedback generation working
- [ ] Emergency controls working
- [ ] Session persistence working

### Performance Requirements
- [ ] Memory usage: <1GB total
- [ ] CPU usage: <50% under normal load
- [ ] Network stability: No connection drops
- [ ] Concurrent processing: Handle multiple drops

---

## 🚨 Known Issues to Monitor

1. **WhatsApp Session Timeout**: Monitor for QR code requests
2. **Google Sheets Rate Limits**: Watch for API quota errors
3. **Database Connection Pool**: Monitor for connection exhaustion
4. **AI API Limits**: Check OpenRouter usage limits
5. **File Permissions**: Ensure session files are accessible

---

## 📝 Test Execution Log

### Execution Date: ___________
### Tester: ___________

| Test Case | Status | Time | Notes |
|-----------|---------|------|-------|
| 1. WhatsApp Auth | ⏸️ | | |
| 2. Drop Detection | ⏸️ | | |
| 3. Sheets Integration | ⏸️ | | |  
| 4. AI Feedback | ⏸️ | | |
| 5. Resubmission | ⏸️ | | |
| 6. Emergency Kill | ⏸️ | | |
| 7. Database Load | ⏸️ | | |
| 8. Recovery | ⏸️ | | |

---

**Next Steps**: Upon successful completion of all test cases, proceed to Phase 3: Cloud Deployment Planning