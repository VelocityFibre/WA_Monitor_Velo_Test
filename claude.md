# Claude Code Operations Guide - Velo Test Project

## 🚀 Quick Start Commands

### **Start All Services (Production Mode)**
```bash
cd projects/velo_test
./scripts/start_all.sh
```

### **Stop All Services**
```bash
cd projects/velo_test
./scripts/stop_all.sh
```

### **Check Service Health**
```bash
cd projects/velo_test
./scripts/health_check.sh
```

---

## 📋 Service Architecture Overview

The Velo Test project operates as a **self-contained microservices system** with the following components:

### **Core Services**

1. **WhatsApp Bridge Service** (Port 8080)
   - Go-based WhatsApp Web integration
   - Handles QR code authentication
   - Captures messages from Velo Test WhatsApp group
   - **Start**: `./scripts/start_whatsapp_bridge.sh`

2. **Drop Monitor Service** (Port 8082)
   - Python-based real-time drop detection
   - Regex pattern matching for DR######## numbers
   - Automatic Google Sheets integration
   - **Start**: `./scripts/start_drop_monitor.sh`

3. **QA Feedback Communicator** (Background Service)
   - Monitors QA review database
   - Generates AI-powered feedback messages
   - Sends WhatsApp notifications to agents
   - **Start**: `./scripts/start_qa_feedback.sh`

4. **Done Message Detector** (Background Service)
   - Detects "DR######## DONE" resubmissions
   - Updates completion status in systems
   - **Start**: `./scripts/start_done_detector.sh`

---

## 🔧 Environment Setup

### **1. Initial Configuration**
```bash
cd projects/velo_test
cp .env.template .env
# Edit .env with your actual credentials
```

### **2. Required Environment Variables**
```bash
# Database Configuration
NEON_DATABASE_URL=postgresql://username:password@host/database

# Google Sheets Integration
GOOGLE_SHEETS_CREDENTIALS_PATH=/path/to/credentials.json
GOOGLE_SHEETS_ID=your_spreadsheet_id

# AI Configuration
LLM_API_KEY=your_openrouter_api_key
LLM_MODEL=x.ai/grok-2-1212:free

# WhatsApp Configuration
VELO_TEST_GROUP_JID=120363421664266245@g.us
```

### **3. Dependencies Installation**
```bash
# Python dependencies
cd services/
pip install -r requirements.txt

# Go dependencies (WhatsApp bridge)
cd whatsapp-bridge/
go mod download
go mod tidy
```

---

## 📁 Directory Structure

```
projects/velo_test/
├── 📂 services/                    # All service implementations
│   ├── whatsapp-bridge/           # Go WhatsApp bridge
│   ├── realtime_drop_monitor.py   # Drop detection service
│   ├── qa_feedback_communicator.py # QA feedback service
│   ├── done_message_detector.py   # Resubmission handler
│   └── velo_test_service.py       # Velo test wrapper
├── 📂 scripts/                     # Automation scripts
├── 📂 config/                      # Configuration files
├── 📂 docs/                        # Documentation
├── 📂 logs/                        # Service logs
├── 📂 docker-data/                 # Docker volumes
│   ├── whatsapp-sessions/         # WhatsApp session storage
│   ├── bridge-logs/               # Bridge service logs
│   └── monitor-logs/              # Monitor service logs
├── 📄 .env.template               # Environment template
├── 📄 docker-compose.yml          # Docker orchestration
└── 📄 README_DEPLOYMENT.md        # Deployment guide
```

---

## 🛠️ Available Scripts

### **Service Management**
- `./scripts/start_all.sh` - Start all services in correct order
- `./scripts/stop_all.sh` - Graceful shutdown of all services
- `./scripts/restart_all.sh` - Restart all services
- `./scripts/health_check.sh` - Check all service health status

### **Individual Service Scripts**
- `./scripts/start_whatsapp_bridge.sh` - Start WhatsApp bridge only
- `./scripts/start_drop_monitor.sh` - Start drop detection service
- `./scripts/start_qa_feedback.sh` - Start QA feedback service
- `./scripts/start_done_detector.sh` - Start done message detector

### **Utility Scripts**
- `./scripts/setup_environment.sh` - Initial environment setup
- `./scripts/cleanup_logs.sh` - Clean old log files
- `./scripts/backup_data.sh` - Backup configuration and data
- `./scripts/deploy_cloud.sh` - Deploy to cloud infrastructure

---

## 🔄 Workflow Operations

### **Phase 1: Drop Detection Workflow**
```bash
# Trigger: Agent posts "DR8888888" in Velo Test WhatsApp group
# Processing: 9-15 seconds
# Actions:
# 1. WhatsApp bridge captures message
# 2. Drop monitor extracts drop number
# 3. Check for duplicates in database
# 4. Add row to Google Sheets
# 5. Create installation record in Neon database
```

### **Phase 2: QA Feedback Workflow**
```bash
# Trigger: QA team marks Column V (Incomplete) as TRUE in Google Sheets
# Processing: 12 seconds
# Actions:
# 1. QA feedback communicator detects incomplete status
# 2. Query database for missing QA steps
# 3. AI generates contextual feedback message
# 4. Send WhatsApp message to agent
# 5. Log feedback delivery
```

### **Phase 3: Resubmission Detection Workflow**
```bash
# Trigger: Agent posts "DR8888888 DONE" in WhatsApp group
# Processing: 15-30 seconds
# Actions:
# 1. Done message detector detects pattern
# 2. Validate drop number exists
# 3. Update Google Sheets Column W to TRUE
# 4. Log resubmission event
# 5. Notify QA team of resubmission
```

---

## 🐍 Python Services Management

### **Using UV Package Manager (Recommended)**
```bash
# Install UV
curl -LsSf https://astral.sh/uv/install.sh | sh

# Create virtual environment
cd projects/velo_test
uv venv --python 3.11
source .venv/bin/activate

# Install dependencies
uv pip install -r services/requirements.txt
```

### **Using Traditional pip**
```bash
cd projects/velo_test
python3 -m venv .venv
source .venv/bin/activate
pip install -r services/requirements.txt
```

---

## 🐹 Go Service Management

### **WhatsApp Bridge Service**
```bash
cd services/whatsapp-bridge/

# Build
go build -o whatsapp-bridge main.go

# Run directly
go run main.go

# Run compiled binary
./whatsapp-bridge
```

---

## 📊 Monitoring & Logging

### **Log Locations**
```bash
# Service logs
tail -f logs/drop_monitor.log          # Drop detection
tail -f logs/qa_feedback.log           # QA feedback
tail -f logs/done_detector.log         # Done message detection
tail -f logs/whatsapp_bridge.log       # WhatsApp bridge

# Docker logs (if using Docker)
docker-compose logs -f whatsapp-bridge
docker-compose logs -f drop-monitor
```

### **Health Check Endpoints**
```bash
# WhatsApp Bridge
curl http://localhost:8080/health

# Drop Monitor Service
curl http://localhost:8082/health

# System Health
./scripts/health_check.sh
```

---

## 🐳 Docker Operations

### **Start with Docker Compose**
```bash
cd projects/velo_test
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### **Docker Service Management**
```bash
# Rebuild specific service
docker-compose build whatsapp-bridge
docker-compose up -d whatsapp-bridge

# Scale services
docker-compose up -d --scale drop-monitor=2
```

---

## ☁️ Cloud Deployment Preparation

### **Pre-Deployment Checklist**
```bash
# 1. Test all services locally
./scripts/health_check.sh

# 2. Validate environment configuration
./scripts/validate_config.sh

# 3. Backup current data
./scripts/backup_data.sh

# 4. Create deployment package
./scripts/create_deployment_package.sh
```

### **Cloud Deployment**
```bash
# Deploy to cloud infrastructure
./scripts/deploy_cloud.sh

# Monitor cloud deployment
./scripts/monitor_cloud_deployment.sh
```

---

## 🚨 Troubleshooting

### **Common Issues & Solutions**

#### **WhatsApp Bridge Not Connecting**
```bash
# Clear session data
rm -rf docker-data/whatsapp-sessions/*

# Restart bridge
./scripts/restart_whatsapp_bridge.sh

# Check QR code within 60 seconds
```

#### **Drop Detection Not Working**
```bash
# Check logs
tail -f logs/drop_monitor.log

# Verify WhatsApp group JID
grep "VELO_TEST_GROUP_JID" .env

# Test pattern matching
python3 -c "import re; print(re.findall(r'DR\d+', 'DR8888888 test'))"
```

#### **Google Sheets Integration Issues**
```bash
# Verify credentials file
ls -la $GOOGLE_SHEETS_CREDENTIALS_PATH

# Test Google Sheets API
python3 -c "
from google.oauth2.service_account import Credentials
from googleapiclient.discovery import build
creds = Credentials.from_service_account_file('$GOOGLE_SHEETS_CREDENTIALS_PATH')
service = build('sheets', 'v4', credentials=creds)
print('Google Sheets API accessible')
"
```

#### **Database Connection Issues**
```bash
# Test Neon database connection
python3 -c "
import psycopg2
conn = psycopg2.connect('$NEON_DATABASE_URL')
print('Database connection successful')
conn.close()
"
```

---

## 📞 Emergency Commands

### **Stop All Services Immediately**
```bash
./scripts/emergency_stop.sh
# OR
pkill -f "whatsapp-bridge\|drop_monitor\|qa_feedback"
```

### **Kill Switch (WhatsApp Trigger)**
Any user in the monitored WhatsApp group can trigger emergency stop by posting:
- `KILL`
- `!KILL`
- `kill all services`
- `emergency stop`

### **Service Recovery**
```bash
# Graceful restart
./scripts/restart_all.sh

# Full reset (last resort)
./scripts/full_reset.sh
```

---

## 🔄 Development Workflow

### **Making Changes**
```bash
# 1. Stop affected services
./scripts/stop_service.sh <service_name>

# 2. Make changes to code
# Edit files in services/ directory

# 3. Restart service
./scripts/start_service.sh <service_name>

# 4. Test changes
./scripts/test_service.sh <service_name>
```

### **Testing New Features**
```bash
# Run in development mode
export DEBUG_MODE=true
export LOG_LEVEL=DEBUG

# Start with debug logging
./scripts/start_all_debug.sh
```

---

## 📈 Performance Optimization

### **Monitoring Performance**
```bash
# Check system resources
./scripts/performance_monitor.sh

# Analyze log patterns
./scripts/analyze_logs.sh

# Database performance
./scripts/check_db_performance.sh
```

### **Optimization Commands**
```bash
# Optimize database
./scripts/optimize_database.sh

# Clean old logs
./scripts/cleanup_logs.sh

# Update dependencies
./scripts/update_dependencies.sh
```

---

## 🎯 Success Metrics

### **Target Performance**
- Drop Detection Speed: <15 seconds ✅
- QA Feedback Response: <30 seconds ✅
- End-to-End Processing: <60 seconds ✅
- Service Uptime: >99% ✅

### **Monitoring Commands**
```bash
# Check real-time metrics
./scripts/show_metrics.sh

# Generate performance report
./scripts/performance_report.sh
```

---

## 📞 Support & Maintenance

### **Regular Maintenance**
```bash
# Daily health check
./scripts/daily_health_check.sh

# Weekly backup
./scripts/weekly_backup.sh

# Monthly maintenance
./scripts/monthly_maintenance.sh
```

### **Getting Help**
```bash
# Show system status
./scripts/system_status.sh

# Generate debug package
./scripts/create_debug_package.sh

# View service logs
./scripts/view_logs.sh
```

---

## 🚨 Railway Deployment Notes (Oct 17, 2025)

### **Persistence Fix Applied**
- **Issue**: WhatsApp sessions required QR code scanning on every Railway restart
- **Solution**: Implemented Railway volume persistence (`/app/store`)
- **Files Modified**: `railway.toml`, `start-services.sh`
- **Result**: WhatsApp session now persists across deployments automatically

### **Key Changes**
```toml
# railway.toml - Added volume mounting
[volumes]
data = "/app/store"

[variables]
RAILWAY_RUN_UID = "0"  # Volume write permissions
```

```bash
# start-services.sh - Simplified persistence
mkdir -p /app/store /app/logs
ln -sf /app/store ./store
ln -sf /app/logs ./logs
```

### **Deployment Process**
1. Deploy updated code to Railway
2. Scan QR code **once only** on first deployment
3. Session persists automatically in `/app/store/whatsapp.db`
4. Future deployments connect without QR codes

---

**🎉 This is your complete operational guide for managing the Velo Test project. All scripts and commands are designed to be run from the `projects/velo_test/` directory.**

*Last Updated: October 17, 2025*

https://github.com/VelocityFibre/WA_Monitor_Velo_Test
