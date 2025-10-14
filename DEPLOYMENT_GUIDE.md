# WhatsApp Monitor Deployment Guide

## Overview

This guide provides step-by-step instructions for deploying the Velo Test WhatsApp Monitoring System locally and in the cloud. The system includes persistent WhatsApp authentication, drop number detection, Google Sheets integration, and resubmission handling.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Prerequisites](#prerequisites)
3. [Local Development Setup](#local-development-setup)
4. [WhatsApp Authentication Setup](#whatsapp-authentication-setup)
5. [Docker Deployment](#docker-deployment)
6. [Cloud Deployment](#cloud-deployment)
7. [Session Persistence](#session-persistence)
8. [Troubleshooting](#troubleshooting)
9. [Backup & Recovery](#backup--recovery)

## System Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  WhatsApp Web   │    │   Bridge Service │    │  Monitor Services│
│                 │◄──►│                  │◄──►│                 │
│  Authentication │    │  Port 8080       │    │  Drop Detection │
└─────────────────┘    └──────────────────┘    │  QA Feedback    │
                                               │  Done Detection │
                       ┌──────────────────┐    └─────────────────┘
                       │   Data Storage   │              │
                       │                  │◄─────────────┘
                       │ • SQLite (local) │
                       │ • PostgreSQL     │
                       │ • Google Sheets  │
                       └──────────────────┘
```

## Prerequisites

### Required Services
- **Neon PostgreSQL Database**: Cloud PostgreSQL instance
- **Google Sheets API**: Service account credentials
- **WhatsApp Account**: For authentication and monitoring

### Software Dependencies
- Docker & Docker Compose
- Python 3.11+
- Go 1.19+ (for WhatsApp bridge)

## Local Development Setup

### 1. Clone and Setup

```bash
cd /path/to/your/deployment
cp .env.template .env
```

### 2. Configure Environment Variables

Edit `.env` file with your credentials:

```bash
# Database Configuration
NEON_DATABASE_URL=postgresql://user:pass@host/db?sslmode=require

# Google Sheets Integration
GSHEET_ID=your_google_sheet_id
GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json

# WhatsApp Groups
VELO_TEST_GROUP_JID=120363421664266245@g.us

# Session Persistence
WHATSAPP_DB_PATH=/path/to/messages.db
```

### 3. Install Dependencies

```bash
# Install Python dependencies
pip install -r requirements.txt

# Build WhatsApp bridge (if needed)
cd services/whatsapp-bridge
go build -o whatsapp-bridge main.go
```

## WhatsApp Authentication Setup

### Critical: One-Time Authentication Process

The WhatsApp session must be authenticated once and then preserved for all future deployments.

### 1. Initial Authentication

```bash
# Start the WhatsApp bridge
cd services/whatsapp-bridge
./whatsapp-bridge
```

**🔍 Look for this output:**
- If you see a **QR Code**: Scan it with WhatsApp
- If you see **"Successfully authenticated"**: Session is working

### 2. Session File Locations

After successful authentication, these files are created:
```
services/whatsapp-bridge/store/
├── whatsapp.db      # Authentication data
└── messages.db      # Message history
```

### 3. Session Persistence Strategy

**For Docker Deployment:**
```bash
# Create persistent volume
docker volume create whatsapp-sessions

# Mount volume in container
-v whatsapp-sessions:/app/services/whatsapp-bridge/store
```

**For Cloud Deployment:**
```bash
# Use cloud storage volumes (AWS EBS, GCP Persistent Disk, etc.)
# Mount to: /app/services/whatsapp-bridge/store
```

## Docker Deployment

### 1. Build Images

```bash
# Build all services
docker-compose build
```

### 2. Environment Setup

Ensure `.env` file is properly configured:

```bash
# Copy authenticated session to Docker volume
docker volume create whatsapp-sessions
docker run --rm -v whatsapp-sessions:/target \
  -v "$(pwd)/services/whatsapp-bridge/store":/source \
  alpine sh -c "cp -r /source/* /target/"
```

### 3. Start Services

```bash
# Start all services
docker-compose up -d

# Check health
docker-compose logs -f
```

### 4. Verify Authentication

```bash
# Check WhatsApp bridge logs
docker-compose logs whatsapp-bridge | grep -E "(Successfully authenticated|Connected to WhatsApp)"

# Test health endpoint
curl http://localhost:8080/health
```

## Cloud Deployment

### AWS Deployment

#### 1. ECS with Fargate

```bash
# Create task definition
aws ecs register-task-definition --cli-input-json file://aws/task-definition.json

# Create service
aws ecs create-service \
  --cluster wa-monitor-cluster \
  --service-name wa-monitor \
  --task-definition wa-monitor:1 \
  --desired-count 1
```

#### 2. Volume Configuration

```json
{
  "mountPoints": [
    {
      "sourceVolume": "whatsapp-sessions",
      "containerPath": "/app/services/whatsapp-bridge/store"
    }
  ],
  "volumes": [
    {
      "name": "whatsapp-sessions",
      "efsVolumeConfiguration": {
        "fileSystemId": "fs-xxxxxxxxx",
        "transitEncryption": "ENABLED"
      }
    }
  ]
}
```

### Google Cloud Deployment

#### 1. Cloud Run

```bash
# Build and push image
gcloud builds submit --tag gcr.io/PROJECT-ID/wa-monitor

# Deploy with persistent disk
gcloud run deploy wa-monitor \
  --image gcr.io/PROJECT-ID/wa-monitor \
  --add-volume name=whatsapp-sessions,type=cloud-storage,bucket=wa-sessions-bucket \
  --add-volume-mount volume=whatsapp-sessions,mount-path=/app/services/whatsapp-bridge/store
```

### Azure Deployment

#### 1. Container Instances

```bash
# Create container group with volume
az container create \
  --resource-group wa-monitor-rg \
  --name wa-monitor \
  --image wa-monitor:latest \
  --azure-file-volume-account-name mystorageaccount \
  --azure-file-volume-account-key $STORAGE_KEY \
  --azure-file-volume-share-name whatsapp-sessions \
  --azure-file-volume-mount-path /app/services/whatsapp-bridge/store
```

## Session Persistence

### Critical Success Factors

1. **Session Files Must Persist**: `whatsapp.db` and `messages.db` must survive container restarts
2. **Volume Mounting**: Always mount `/app/services/whatsapp-bridge/store` as a persistent volume
3. **Backup Strategy**: Regular backups of session files to prevent re-authentication

### Session Validation

```bash
# Check if session is valid
ls -la /app/services/whatsapp-bridge/store/
# Should show:
# whatsapp.db (authentication data)
# messages.db (message history)

# Test connection without QR code
docker run --rm \
  -v whatsapp-sessions:/app/services/whatsapp-bridge/store \
  wa-monitor:latest \
  ./services/whatsapp-bridge/whatsapp-bridge
```

**Expected Output (Success):**
```
Successfully authenticated
Connected to WhatsApp
REST server is running
```

**Expected Output (Needs Re-auth):**
```
Scan this QR code with your WhatsApp app:
[QR CODE DISPLAY]
```

## Troubleshooting

### WhatsApp Authentication Issues

#### Problem: QR Code Always Appears
```bash
# Solution 1: Check session files
ls -la services/whatsapp-bridge/store/
# Files should exist and be recent

# Solution 2: Copy from working session
cp /path/to/working/whatsapp.db services/whatsapp-bridge/store/
cp /path/to/working/messages.db services/whatsapp-bridge/store/

# Solution 3: Re-authenticate (last resort)
rm services/whatsapp-bridge/store/*.db
./services/whatsapp-bridge/whatsapp-bridge
# Scan QR code when displayed
```

#### Problem: "Session Expired" Errors
```bash
# This happens after long inactivity
# Solution: Re-authenticate with QR code
# The session files will be updated automatically
```

### Service Connection Issues

#### Problem: Database Connection Errors
```bash
# Check environment variables
echo $WHATSAPP_DB_PATH
echo $NEON_DATABASE_URL

# Test database connection
python3 -c "
import sqlite3
conn = sqlite3.connect('$WHATSAPP_DB_PATH')
print('SQLite OK')
conn.close()
"
```

#### Problem: Google Sheets Access Denied
```bash
# Check credentials file
ls -la credentials.json

# Test credentials
python3 -c "
from google.oauth2.service_account import Credentials
creds = Credentials.from_service_account_file('credentials.json')
print('Credentials OK')
"
```

## Backup & Recovery

### Session Backup

```bash
# Create session backup
mkdir -p backups/$(date +%Y%m%d)
cp services/whatsapp-bridge/store/*.db backups/$(date +%Y%m%d)/

# Cloud backup (example: AWS S3)
aws s3 sync services/whatsapp-bridge/store/ s3://wa-monitor-backups/sessions/$(date +%Y%m%d)/
```

### Session Recovery

```bash
# Restore from backup
cp backups/YYYYMMDD/*.db services/whatsapp-bridge/store/

# Set correct permissions
chown -R app:app services/whatsapp-bridge/store/
chmod 644 services/whatsapp-bridge/store/*.db
```

### Disaster Recovery Procedure

1. **Stop all services**
2. **Restore session files** from latest backup
3. **Start WhatsApp bridge** first
4. **Verify authentication** (should connect without QR)
5. **Start monitoring services**
6. **Test system** with a drop number

## Production Checklist

### Before Cloud Deployment

- [ ] WhatsApp authenticated locally and session files backed up
- [ ] All environment variables configured correctly
- [ ] Google Sheets API access tested
- [ ] Database connections verified
- [ ] Docker images built and tested locally
- [ ] Session persistence volumes configured
- [ ] Health checks implemented
- [ ] Monitoring and alerting set up

### After Cloud Deployment

- [ ] Services started successfully
- [ ] WhatsApp bridge connected without QR code
- [ ] Health endpoints responding
- [ ] Test drop number detection working
- [ ] Google Sheets integration functional
- [ ] Session files backed up to cloud storage
- [ ] Monitoring alerts configured

## Security Considerations

1. **Session Files**: Contains WhatsApp authentication - treat as secrets
2. **Database Credentials**: Use secure credential management
3. **API Keys**: Store in secure environment variables or secret managers
4. **Network Security**: Restrict access to bridge port 8080
5. **Backup Security**: Encrypt session backups

## Performance Tuning

1. **Resource Allocation**:
   - WhatsApp Bridge: 512MB RAM minimum
   - Monitoring Services: 256MB RAM each
   - CPU: 0.5 cores per service minimum

2. **Scaling Considerations**:
   - Single WhatsApp bridge instance only (authentication limitation)
   - Monitor services can be scaled horizontally
   - Database connection pooling recommended

## Support & Maintenance

### Log Locations
```
logs/
├── whatsapp_bridge.log
├── drop_monitor.log
├── qa_feedback.log
└── done_detector.log
```

### Health Check Endpoints
- WhatsApp Bridge: `http://localhost:8080/health`
- System Status: `./scripts/health_check.sh`

### Regular Maintenance
1. **Weekly**: Check session file integrity
2. **Monthly**: Backup session files
3. **Quarterly**: Update dependencies and security patches

---

**Last Updated**: October 2025  
**Version**: 3.0.0  
**Contact**: VF Operations Team