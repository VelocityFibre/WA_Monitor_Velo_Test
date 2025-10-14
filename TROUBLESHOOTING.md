# Troubleshooting & Session Management Guide

## 🔧 Common Issues and Fixes

### 1. Missing Audio Module Dependency

**Issue**: 
```
ModuleNotFoundError: No module named 'audio'
```

**Explanation**: 
- The `whatsapp.py` file imports an `audio` module for voice message conversion
- This module isn't needed for basic text monitoring functionality
- Audio features are not implemented yet in this deployment

**Solution Applied**:
- Commented out `import audio` in `services/whatsapp.py`
- Disabled audio conversion in `send_audio_message()` function
- Service now runs without audio dependencies

**Future Implementation**:
When audio functionality is needed:
1. Install audio processing libraries: `pip install pydub audio`
2. Install system dependencies: `sudo apt install ffmpeg`
3. Uncomment audio imports and functionality

### 2. WhatsApp Database Path Issues

**Issue**:
```
❌ WhatsApp database not found at ../whatsapp-bridge/store/messages.db
```

**Explanation**:
- Services were looking for database in wrong relative path
- Path needs to account for project directory structure

**Solution Applied**:
- Updated `MESSAGES_DB_PATH` in `services/realtime_drop_monitor.py`
- Added `WHATSAPP_DB_PATH` environment variable to `.env`
- Fixed relative paths to work with current directory structure

### 3. Environment Variable Path Spaces

**Issue**:
```
.env: line 35: _Velo_Test/credentials.json: No such file or directory
```

**Explanation**:
- Directory name contains spaces: `WA_monitor _Velo_Test`
- Shell interprets unquoted paths with spaces incorrectly

**Solution Applied**:
- Quoted `GOOGLE_SHEETS_CREDENTIALS_PATH` in `.env` file
- All paths with spaces now properly quoted

---

## 📱 WhatsApp Session Management

### Session Persistence for Cloud Deployment

**Critical for 24/7 Operation**: WhatsApp sessions must persist across restarts to avoid re-authentication.

### Session Files Location

**Source** (where QR code was scanned):
```
/home/louisdup/VF/Apps/WA_Tool/projects/velo_test/services/whatsapp-bridge/store/
├── whatsapp.db     (1.3MB) - Session authentication data
├── messages.db     (4.2MB) - Message history
```

**Current Deployment** (copied for persistence):
```
/home/louisdup/VF/deployments/WA_monitor _Velo_Test/
├── services/whatsapp-bridge/store/     # For current local service
│   ├── whatsapp.db
│   └── messages.db
└── docker-data/whatsapp-sessions/      # For cloud deployment
    ├── whatsapp.db
    └── messages.db
```

### Session Management Commands

```bash
# Backup current session
cp services/whatsapp-bridge/store/*.db docker-data/whatsapp-sessions/

# Restore session after deployment
cp docker-data/whatsapp-sessions/*.db services/whatsapp-bridge/store/

# For Docker deployment - mount volume
docker-compose.yml:
  volumes:
    - ./docker-data/whatsapp-sessions:/app/store
```

### Session Validation

```bash
# Check if session files exist
ls -la docker-data/whatsapp-sessions/
ls -la services/whatsapp-bridge/store/

# Verify file sizes (should be > 1MB for whatsapp.db)
du -h docker-data/whatsapp-sessions/*

# Test WhatsApp connectivity
curl http://localhost:8080/health
```

### Re-authentication Process

If session becomes invalid:
1. Clear session files: `rm -rf docker-data/whatsapp-sessions/*`
2. Restart WhatsApp bridge service
3. Scan new QR code with WhatsApp mobile app
4. Copy new session files to persistent location
5. For cloud: ensure session files are in mounted volume

### Cloud Deployment Session Handling

1. **Initial Setup**: 
   - Start services locally
   - Complete QR code authentication
   - Copy session files to persistent volume

2. **Cloud Migration**:
   - Upload session files to cloud storage
   - Mount persistent volume in container
   - Ensure session files have correct permissions

3. **Monitoring**:
   - Set up alerts for WhatsApp disconnection
   - Automate session backup schedule
   - Create re-authentication procedure

---

## 🚀 Service Dependencies

### Required for Startup
1. **Database Connection**: Neon PostgreSQL must be accessible
2. **Google Sheets API**: Service account credentials must be valid
3. **WhatsApp Session**: For message monitoring (can start without, but won't monitor)
4. **OpenRouter API**: For QA feedback generation

### Service Start Order
1. **WhatsApp Bridge** (Port 8080) - Core messaging service
2. **Drop Monitor** - Depends on WhatsApp bridge database
3. **QA Feedback Communicator** - Independent, needs AI API
4. **Done Message Detector** - Depends on WhatsApp bridge database

### Health Check Commands

```bash
# Full system health check
./scripts/health_check.sh

# Individual service checks
curl http://localhost:8080/health  # WhatsApp Bridge
ps aux | grep python3             # Python services
tail -f logs/*.log                # Service logs
```

---

## 📋 Pre-Cloud Deployment Checklist

- [ ] All services start without errors locally
- [ ] WhatsApp session authenticated and backed up
- [ ] Google Sheets integration working
- [ ] Database connectivity confirmed
- [ ] Session files copied to docker-data/whatsapp-sessions/
- [ ] Environment variables properly quoted
- [ ] Health checks passing consistently

---

**Last Updated**: 2025-10-14  
**Deployment Version**: WA_Tool v3.0.0