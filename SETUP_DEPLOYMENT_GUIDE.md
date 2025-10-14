# Complete Setup & Deployment Guide

**Velo Test WhatsApp Monitor - From Local Setup to Cloud Deployment**

Date: 2025-10-14  
Version: WA_Tool v3.0.0  
Status: ✅ Production Ready

---

## 🏗️ What We Built

### **Microservices Architecture**
- **WhatsApp Bridge** (Go) - WhatsApp Web integration on port 8080
- **Drop Monitor** (Python) - Real-time DR number detection
- **QA Feedback Communicator** (Python) - AI-powered feedback system
- **Done Message Detector** (Python) - Resubmission handling
- **Management Scripts** - Reliable service control

### **Key Integrations**
- **Neon PostgreSQL** - Primary database
- **Google Sheets API** - Bidirectional data sync
- **OpenRouter AI** - Automated QA feedback generation
- **WhatsApp Web** - Message monitoring and sending

---

## 📋 Prerequisites

### **System Requirements**
- **OS**: Ubuntu 20.04+ (tested on Ubuntu)
- **RAM**: 4GB minimum, 8GB recommended
- **Storage**: 5GB free space
- **Network**: Stable internet connection

### **Software Dependencies**
```bash
# Required software
sudo apt update && sudo apt install -y curl git python3 python3-pip python3-venv golang-go lsof
```

### **External Services Required**
1. **Neon PostgreSQL Database** - Cloud database
2. **Google Cloud Project** - For Sheets API access
3. **OpenRouter Account** - For AI feedback generation
4. **WhatsApp Account** - For group monitoring

---

## 🚀 Complete Local Setup Process

### **Step 1: Initial Project Setup**

```bash
# Clone or copy project files
cd /path/to/your/projects/
# [Assume project files are already in place]

# Navigate to project directory
cd WA_monitor_Velo_Test

# Verify project structure
ls -la
# Should show: services/, scripts/, docker/, logs/, .env.template, etc.
```

### **Step 2: Environment Configuration**

```bash
# Copy environment template
cp .env.template .env

# Edit environment file with your actual values
nano .env
```

**Critical Environment Variables:**
```bash
# Database Configuration
NEON_DATABASE_URL="postgresql://username:password@host/database?sslmode=require"

# Google Sheets Integration  
GOOGLE_SHEETS_CREDENTIALS_PATH="/full/path/to/credentials.json"
GOOGLE_SHEETS_ID="your_spreadsheet_id_from_url"

# AI Configuration
LLM_API_KEY="your_openrouter_api_key"
LLM_MODEL="x.ai/grok-2-1212:free"

# WhatsApp Configuration
VELO_TEST_GROUP_JID="120363421664266245@g.us"

# WhatsApp Database Path (use existing authenticated database)
WHATSAPP_DB_PATH="/home/louisdup/VF/Apps/WA_Tool/whatsapp-mcp/whatsapp-bridge/store/messages.db"
```

### **Step 3: Google Sheets Authentication Setup**

```bash
# 1. Create Google Cloud Project (if needed)
# - Go to https://console.cloud.google.com/
# - Create new project or use existing
# - Enable Google Sheets API and Google Drive API

# 2. Create Service Account
# - Go to IAM & Admin → Service Accounts
# - Create new service account
# - Download JSON credentials file

# 3. Secure credentials installation
cp /path/to/downloaded/credentials.json ./credentials.json

# 4. Verify credentials path in .env matches actual location
grep GOOGLE_SHEETS_CREDENTIALS_PATH .env
```

### **Step 4: Python Environment Setup**

```bash
# Create virtual environment
python3 -m venv .venv

# Activate virtual environment
source .venv/bin/activate

# Install Python dependencies
pip install -r services/requirements.txt

# Verify installation
pip list | grep -E "(psycopg2|google)"
```

### **Step 5: Go Dependencies Setup**

```bash
# Navigate to WhatsApp bridge directory
cd services/whatsapp-bridge

# Download and verify Go dependencies
go mod download
go mod tidy

# Return to project root
cd ../..
```

### **Step 6: Service Management Setup**

```bash
# Make service management script executable
chmod +x manage_services.sh
chmod +x test_system.sh

# Create necessary directories
mkdir -p logs docker-data/whatsapp-sessions docker-data/bridge-logs docker-data/monitor-logs
```

---

## 🛠️ WhatsApp Authentication Setup

### **Option 1: Use Existing Authentication (Recommended)**

```bash
# If you have an existing authenticated WhatsApp bridge:

# 1. Find the active WhatsApp database
find /home/louisdup/VF/Apps/WA_Tool -name "messages.db" -type f 2>/dev/null

# 2. Update .env to point to the active database
WHATSAPP_DB_PATH="/path/to/active/whatsapp/bridge/store/messages.db"

# 3. No QR scan required!
```

### **Option 2: Fresh Authentication**

```bash
# If starting fresh:

# 1. Start WhatsApp Bridge
./manage_services.sh start whatsapp-bridge

# 2. View QR code
tail -30 logs/whatsapp-bridge.log

# 3. Scan QR code with WhatsApp mobile app:
#    - WhatsApp → Settings → Linked Devices → Link a Device

# 4. Wait for connection confirmation in logs
```

---

## 🧪 Local Testing & Validation

### **Start All Services**

```bash
# Start all services
./manage_services.sh start

# Check service status
./manage_services.sh status

# Expected output:
# ✅ drop-monitor is running (PID: XXXXX)
# ✅ done-detector is running (PID: XXXXX)  
# ✅ whatsapp-bridge is running (PID: XXXXX)
# ⏸️ qa-feedback (runs on-demand - normal)
```

### **Run System Tests**

```bash
# Run comprehensive system test
./test_system.sh

# Expected results:
# ✅ Database connectivity working
# ✅ Google Sheets API working
# ✅ Services running properly
# ✅ System resources normal
```

### **Test Individual Components**

```bash
# Test database connection
source .venv/bin/activate && python3 -c "
import psycopg2
import os
conn = psycopg2.connect(os.environ['NEON_DATABASE_URL'])
print('✅ Database connection successful')
conn.close()
"

# Test Google Sheets API
source .venv/bin/activate && python3 -c "
from google.oauth2.service_account import Credentials
from googleapiclient.discovery import build
import os
creds = Credentials.from_service_account_file(os.environ['GOOGLE_SHEETS_CREDENTIALS_PATH'])
service = build('sheets', 'v4', credentials=creds)
print('✅ Google Sheets API working')
"

# Monitor real-time activity
tail -f logs/drop-monitor.log
tail -f logs/done-detector.log
```

### **End-to-End Workflow Test**

```bash
# 1. Test drop detection (if WhatsApp authenticated)
# Post "Testing DR9999999" in Velo Test WhatsApp group

# 2. Monitor logs for detection
tail -f logs/drop-monitor.log

# 3. Check database for new entry
source .venv/bin/activate && python3 -c "
import psycopg2
import os
conn = psycopg2.connect(os.environ['NEON_DATABASE_URL'])
cur = conn.cursor()
cur.execute('SELECT drop_number, created_at FROM drops ORDER BY created_at DESC LIMIT 5;')
print('Recent drops:', cur.fetchall())
conn.close()
"

# 4. Verify Google Sheets integration
# Check Google Sheets for new row with drop data

# 5. Test QA feedback workflow
# Manually mark drop as "Incomplete = TRUE" in Google Sheets
# Check for automated feedback message in WhatsApp group
```

---

## 🐳 Docker Containerization

### **Docker Compose Setup**

The project includes a complete `docker-compose.yml` with:
- All 5 microservices containerized
- Health checks and dependency management
- Volume mounts for persistent data
- Network isolation and service discovery

```bash
# Build and start with Docker
docker-compose up -d

# View logs
docker-compose logs -f

# Check service health
docker-compose ps

# Stop services
docker-compose down
```

### **Key Docker Volumes**
```yaml
# Critical persistent data
- ./docker-data/whatsapp-sessions:/app/store  # WhatsApp session persistence
- ./credentials.json:/app/credentials.json    # Google Sheets credentials
- ./.env:/app/.env                            # Environment configuration
```

### **Docker Service Ports**
- **WhatsApp Bridge**: 8080 (health: `/health`)
- **Velo Test Service**: 8082 (management interface)

---

## ☁️ Cloud Deployment Options

### **Option 1: VPS Deployment (Simplest)**

**Recommended Providers**: DigitalOcean, Linode, Vultr  
**Cost**: $20-40/month for 2vCPU, 4GB RAM

```bash
# 1. Provision VPS with Ubuntu 20.04+

# 2. Connect via SSH
ssh root@your-server-ip

# 3. Install dependencies
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh
apt install docker-compose-plugin

# 4. Deploy project files
rsync -av /local/path/to/WA_monitor_Velo_Test/ root@server:/opt/wa-monitor/
# OR
git clone <your-repo> /opt/wa-monitor

# 5. Configure environment
cd /opt/wa-monitor
cp .env.template .env
nano .env  # Update with production values

# 6. Deploy with Docker
docker-compose up -d

# 7. Verify deployment
docker-compose ps
curl http://localhost:8080/health
```

### **Option 2: AWS Deployment**

**Services**: ECS with Fargate, RDS PostgreSQL, Route53  
**Cost**: $50-100/month

```bash
# 1. Create ECS cluster
aws ecs create-cluster --cluster-name wa-monitor

# 2. Build and push images to ECR
docker build -t wa-monitor/whatsapp-bridge services/whatsapp-bridge/
aws ecr create-repository --repository-name wa-monitor/whatsapp-bridge
docker tag wa-monitor/whatsapp-bridge:latest <ecr-uri>
docker push <ecr-uri>

# 3. Create ECS task definition
# 4. Deploy ECS service with load balancer
# 5. Configure RDS PostgreSQL database
# 6. Set up Route53 domain and SSL
```

### **Option 3: Google Cloud Deployment**

**Services**: Cloud Run, Cloud SQL, Load Balancer  
**Cost**: $40-80/month

```bash
# 1. Build and deploy to Cloud Run
gcloud run deploy wa-monitor \
  --source . \
  --region us-central1 \
  --allow-unauthenticated

# 2. Configure Cloud SQL PostgreSQL
# 3. Set up custom domain and SSL
# 4. Configure secrets management
```

---

## 🔒 Security & Production Considerations

### **Environment Security**

```bash
# 1. Secure credentials file permissions
chmod 600 credentials.json
chown app:app credentials.json

# 2. Use environment secrets (not files) in production
# Set via docker-compose environment or cloud secrets manager

# 3. Network security
# - Use VPC with private subnets
# - Restrict inbound traffic to necessary ports only
# - Enable SSL/TLS for all external communications
```

### **Backup Strategy**

```bash
# 1. WhatsApp session backup
cp docker-data/whatsapp-sessions/* /backup/location/

# 2. Database backup (automated)
pg_dump $NEON_DATABASE_URL > backup_$(date +%Y%m%d).sql

# 3. Configuration backup
tar -czf config-backup.tar.gz .env credentials.json docker-compose.yml
```

### **Monitoring & Alerts**

```bash
# 1. Health check monitoring
# Set up monitoring service to check:
curl http://your-server:8080/health  # WhatsApp Bridge
curl http://your-server:8082/health  # Management Service

# 2. Log monitoring
# - Aggregate logs to central system (ELK stack, CloudWatch)
# - Set up alerts for ERROR and CRITICAL log levels

# 3. Resource monitoring  
# - CPU, RAM, disk usage alerts
# - Network connectivity monitoring
# - Database connection health
```

---

## 🛠️ Troubleshooting Guide

### **Common Issues & Solutions**

**Service Won't Start:**
```bash
# Check logs
./manage_services.sh logs service-name

# Check dependencies
./test_system.sh

# Restart individual service
./manage_services.sh restart service-name
```

**WhatsApp Authentication Issues:**
```bash
# Clear session and re-authenticate
rm -rf docker-data/whatsapp-sessions/*
./manage_services.sh restart whatsapp-bridge
# Scan new QR code
```

**Database Connection Issues:**
```bash
# Test connection
source .venv/bin/activate
python3 -c "import psycopg2; psycopg2.connect('$NEON_DATABASE_URL')"

# Check environment variables
grep NEON_DATABASE_URL .env
```

**Google Sheets Permission Errors:**
```bash
# Verify service account has Editor access to spreadsheet
# Check credentials file exists and has correct permissions
ls -la credentials.json
```

---

## 📊 Success Metrics

### **Local Testing Success:**
- [ ] All services start without errors
- [ ] Health checks pass consistently  
- [ ] Database connectivity stable
- [ ] Google Sheets integration working
- [ ] WhatsApp session persistent across restarts
- [ ] Drop detection working (test message processed)
- [ ] QA feedback generation working
- [ ] Emergency kill switch functional

### **Cloud Deployment Success:**
- [ ] All services deployed and healthy
- [ ] 99.9% uptime achieved
- [ ] Response time < 30 seconds for all operations
- [ ] Auto-restart working after failures
- [ ] Monitoring and alerts configured
- [ ] Backups automated and tested
- [ ] SSL/TLS security implemented
- [ ] Cost within budget ($50-150/month)

---

## 🎯 Quick Commands Reference

### **Service Management**
```bash
./manage_services.sh start           # Start all services
./manage_services.sh stop            # Stop all services  
./manage_services.sh restart         # Restart all services
./manage_services.sh status          # Show service status
./manage_services.sh logs [service]  # View service logs
./manage_services.sh health          # Run health checks
```

### **Testing & Validation**
```bash
./test_system.sh                     # Run comprehensive system test
tail -f logs/drop-monitor.log        # Monitor drop detection
tail -f logs/qa-feedback.log         # Monitor QA feedback
tail -f logs/whatsapp-bridge.log     # Monitor WhatsApp connection
```

### **Docker Operations**
```bash
docker-compose up -d                 # Start all services
docker-compose down                  # Stop all services
docker-compose logs -f               # View all logs
docker-compose ps                    # Show service status
docker-compose restart service-name  # Restart specific service
```

---

## 📝 Version History

**v3.0.0** (2025-10-14) - Current
- Complete microservices architecture
- Reliable service management system
- WhatsApp session persistence solved
- Docker containerization ready
- Cloud deployment prepared
- Comprehensive documentation

---

**🚀 Ready for Production Deployment!**

This guide provides everything needed to replicate the setup locally and deploy to any cloud platform. The system is production-ready with robust service management, monitoring, and recovery mechanisms.

**Next Action**: Choose cloud deployment option and execute deployment plan.