# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Overview

This repository contains the **Velo Test WhatsApp Monitoring System** - a self-contained microservices deployment for automated drop number detection, QA feedback automation, and resubmission handling for telecom installations. The system monitors WhatsApp groups, syncs data with Google Sheets and Neon PostgreSQL database, and provides AI-powered feedback to installation agents.

## Architecture

### Core Microservices
- **WhatsApp Bridge** (Go, Port 8080): WhatsApp Web integration and message capture
- **Drop Monitor** (Python): Real-time drop number detection (regex: `DR\d+`)
- **QA Feedback Communicator** (Python): AI-powered feedback generation and delivery
- **Done Message Detector** (Python): Resubmission detection (`DR######## DONE`)
- **Velo Test Service** (Python, Port 8082): Management and monitoring wrapper

### Data Flow
1. **Drop Detection**: Agent posts `DR########` → Bridge captures → Monitor processes → Google Sheets + Neon DB
2. **QA Feedback**: QA marks incomplete in sheets → AI generates feedback → WhatsApp message to agent
3. **Resubmission**: Agent posts `DR######## DONE` → Detector validates → Updates completion status

### Technology Stack
- **Backend**: Python 3.11+ (services), Go 1.19+ (bridge)
- **Database**: Neon PostgreSQL (primary), SQLite (local WhatsApp store)
- **Integrations**: Google Sheets API, OpenRouter AI, WhatsApp Web
- **Deployment**: Docker Compose with health checks

## Essential Commands

### Local Development

```bash
# Initial setup
cp .env.template .env
# Edit .env with your credentials
./scripts/setup_environment.sh

# Start all services
./scripts/start_all.sh

# Check service health
./scripts/health_check.sh

# Stop all services
./scripts/stop_all.sh
```

### Docker Deployment

```bash
# Start with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Individual Service Management

```bash
# Python services (activate venv first)
source .venv/bin/activate
python3 services/realtime_drop_monitor.py
python3 services/qa_feedback_communicator.py
python3 services/done_message_detector.py

# Go WhatsApp bridge
cd services/whatsapp-bridge
go run main.go
```

### Testing Commands

```bash
# Test database connection
python3 -c "
import psycopg2
conn = psycopg2.connect('$NEON_DATABASE_URL')
print('✅ Database connection successful')
conn.close()
"

# Test Google Sheets API
python3 -c "
from google.oauth2.service_account import Credentials
from googleapiclient.discovery import build
creds = Credentials.from_service_account_file('$GOOGLE_SHEETS_CREDENTIALS_PATH')
service = build('sheets', 'v4', credentials=creds)
print('✅ Google Sheets API accessible')
"

# Validate drop number regex
python3 -c "
import re
print(re.findall(r'DR\\d+', 'DR8888888 test message'))
"
```

## Configuration Requirements

### Critical Environment Variables
- `NEON_DATABASE_URL`: PostgreSQL connection string
- `GOOGLE_SHEETS_CREDENTIALS_PATH`: Service account JSON file path
- `GOOGLE_SHEETS_ID`: Target spreadsheet ID
- `LLM_API_KEY`: OpenRouter API key for AI feedback
- `VELO_TEST_GROUP_JID`: WhatsApp group identifier (120363421664266245@g.us)

### Service Ports
- WhatsApp Bridge: 8080 (health endpoint: `/health`)
- Velo Test Service: 8082 (optional management interface)

### Google Sheets Integration
Expected columns in spreadsheet:
- Column V: `Incomplete` (TRUE/FALSE for QA status)
- Column W: `Resubmitted` (TRUE/FALSE for completion)
- Column X: `Completed` (TRUE/FALSE for final status)

## Development Guidelines

### Code Structure
```
services/
├── whatsapp-bridge/          # Go WhatsApp integration
├── realtime_drop_monitor.py  # Drop detection service
├── qa_feedback_communicator.py # AI feedback service
├── done_message_detector.py  # Resubmission handler
├── whatsapp.py              # WhatsApp API wrapper
└── requirements.txt         # Python dependencies

scripts/                     # Automation scripts
├── start_all.sh            # Service startup
├── stop_all.sh             # Graceful shutdown
├── health_check.sh         # System health validation
└── setup_environment.sh    # Initial setup

docker/                      # Docker configurations
├── Dockerfile.drop_monitor
├── Dockerfile.qa_feedback
├── Dockerfile.done_detector
└── docker-compose.yml
```

### Emergency Procedures
- **Kill Switch**: Any monitored group user can post "KILL", "!KILL", or "emergency stop"
- **Service Recovery**: Use `./scripts/restart_all.sh` for graceful restart
- **WhatsApp Re-auth**: Clear `docker-data/whatsapp-sessions/*` and restart bridge

### Performance Targets
- Drop Detection: <15 seconds
- QA Feedback Response: <30 seconds
- End-to-End Processing: <60 seconds
- Service Uptime: >99%

## Troubleshooting

### WhatsApp Bridge Issues
```bash
# Clear session and restart
rm -rf docker-data/whatsapp-sessions/*
./scripts/restart_whatsapp_bridge.sh
# Scan QR code within 60 seconds
```

### Database Connectivity
```bash
# Test connection
python3 -c "import psycopg2; psycopg2.connect('$NEON_DATABASE_URL')"
```

### Google Sheets Permission
Ensure service account email has Editor access to the target spreadsheet.

### Log Locations
- `logs/drop_monitor.log` - Drop detection activity
- `logs/qa_feedback.log` - QA feedback communication
- `logs/done_detector.log` - Resubmission detection
- `logs/whatsapp_bridge.log` - WhatsApp bridge operations

## Project-Specific Rules

- All drop numbers follow format `DR\d+` (e.g., DR8888888)
- Services must handle graceful shutdown via SIGTERM
- Emergency kill commands take immediate effect
- Feedback cooldown prevents spam (300 seconds default)
- Multi-project support: Velo Test, Lawley, Mohadin groups
- QA feedback uses OpenRouter AI models (default: x.ai/grok-2-1212:free)
- WhatsApp session persistence required across restarts
- Database operations use connection pooling
- All services log to both file and stdout
- Health checks required for production deployment

## Deployment Notes

This is a **production-ready** self-contained deployment based on WA_Tool v3.0.0. It includes:
- Automated workflows for drop detection → QA feedback → resubmission handling
- Cloud deployment capabilities (AWS, GCP, Azure)
- Docker orchestration with health checks
- Comprehensive monitoring and logging
- Emergency procedures and recovery mechanisms

For cloud deployment, validate local setup first using `./scripts/health_check.sh` before proceeding with cloud-specific deployment scripts.