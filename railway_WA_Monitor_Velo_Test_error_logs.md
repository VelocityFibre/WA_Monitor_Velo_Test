# Railway WA Monitor Velo Test - Error Tracking Log

**Date:** 15 October 2025  
**Time:** 13:46 UTC  
**Project:** WA_Monitor_Velo_Test  
**Railway Deployment:** 27a48322  

## Current Status: PERSISTENT DROP MONITOR CRASHES

### ❌ Current Issue
- **WhatsApp Bridge:** ✅ Working correctly (connects automatically, no QR needed)
- **Drop Monitor:** ❌ Continuously crashes and restarts every ~30 seconds
- **QA Feedback Service:** ✅ Appears to be running (PID: 43)

### 🔄 Problem Pattern (Ongoing Loop)
Despite multiple attempts to fix Google Sheets credentials, the Drop Monitor continues to crash with the same pattern:

```
✅ Drop Monitor started (PID: 42)
❌ Drop Monitor crashed, restarting...
🔄 Drop Monitor restarted (PID: 45)
❌ Drop Monitor crashed, restarting...
🔄 Drop Monitor restarted (PID: 47)
[... repeats indefinitely ...]
```

## 📋 Troubleshooting History

### Attempt #1: Path Issues (12:18 PM)
**Problem:** Startup script used `/app/` paths but Railway working directory was different
**Fix:** Changed paths to relative (./store, ./logs, ./credentials.json)  
**Result:** ❌ Still crashing

### Attempt #2: Environment Variable Paths (12:20 PM) 
**Problem:** GOOGLE_APPLICATION_CREDENTIALS pointed to wrong location
**Fix:** Updated to use `$(pwd)/credentials.json`  
**Result:** ❌ Still crashing

### Attempt #3: Railway Path Expectations (13:20 PM)
**Problem:** Railway expects `/app/credentials.json` but we can't write there
**Fix:** Attempted to create at both locations  
**Result:** ❌ Still crashing - `/app` directory not writable

### Attempt #4: Override Environment Variable (13:41 PM) - CURRENT
**Problem:** Railway's GOOGLE_APPLICATION_CREDENTIALS override not working
**Fix:** Properly export GOOGLE_APPLICATION_CREDENTIALS to actual file location  
**Result:** ❌ STILL CRASHING (as of 13:46 PM)

## 🔍 Current Environment Status

### Environment Variables (Confirmed Working)
```bash
GOOGLE_SHEETS_ID=1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk ✅
GOOGLE_APPLICATION_CREDENTIALS=/app/credentials.json ❌ (shows old path in logs)
```

### File Locations (Confirmed)
```bash
./credentials.json - ✅ EXISTS (2379 bytes)
/app/credentials.json - ❌ DOES NOT EXIST (permission denied)
```

### Python Libraries Status
```
✅ Google Sheets libraries available
✅ google-api-python-client installed
✅ google-auth installed
```

## 🚨 URGENT: Need to Check Drop Monitor Logs

**NEXT STEPS (Stop the loop):**

1. **Get actual Drop Monitor error logs** - we keep checking old logs
   ```bash
   railway run -- tail -50 ./logs/drop-monitor.log
   ```

2. **Run Drop Monitor directly** to see real-time error:
   ```bash
   railway run -- python3 services/realtime_drop_monitor.py --dry-run
   ```

3. **Check if environment variables are actually being passed to Python process**

## 🔧 Current Deployment Details

**Railway Deployment ID:** 27a48322  
**Active Since:** Oct 15, 2025, 1:41 PM  
**WhatsApp Connection:** ✅ Auto-connected (session persisted)  
**REST API:** ✅ Running on port 8080  

## 💡 Working Theory

The startup script shows correct environment variables:
```
✅ Credentials file created at /app/credentials.json
🔍 Environment variables set:
  GOOGLE_APPLICATION_CREDENTIALS=/app/credentials.json
  GOOGLE_SHEETS_ID=1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk
```

But the **actual file doesn't exist at that location**. The issue is likely:
1. Environment variable export is happening in shell but not being inherited by Python processes
2. Or there's a different error in the Drop Monitor that's not related to Google Sheets at all

## 🎯 ROOT CAUSE IDENTIFIED (13:51 UTC)

**ACTUAL ERROR DISCOVERED:**
```
❌ WhatsApp database not found at /app/store/messages.db
   Make sure the WhatsApp bridge is running and has created the database.
```

**REVELATION:** The issue is NOT Google Sheets credentials at all!
- Drop Monitor is looking for WhatsApp database at `/app/store/messages.db`
- But database is actually at `./store/messages.db` 
- Our startup script sets `WHATSAPP_DB_PATH="./store/messages.db"` but Drop Monitor doesn't see this environment variable

## 🔧 REAL FIX NEEDED

The Drop Monitor Python code has:
```python
MESSAGES_DB_PATH = os.getenv('WHATSAPP_DB_PATH', '/app/store/messages.db')
```

But the environment variable is not being passed to the Python process!

**Status:** Ready to implement real fix - database path issue, not credentials

---

## 🚀 Railway Performance Fix (13:59 UTC)

**SECONDARY ISSUE IDENTIFIED:** 4+ minute snapshot times due to large files in git
- **Removed:** 33MB+ Go compiled binaries causing slow snapshots
- **Removed:** 2.9MB base64 session file
- **Added:** Comprehensive .gitignore for future prevention
- **Result:** Should dramatically speed up Railway deployments

---

## 🐋 DOCKERFILE FIX (14:42 UTC)

**DEPLOYMENT FAILURE ROOT CAUSE:** Dockerfile trying to COPY missing whatsapp-bridge binary
- **Error:** `"/services/whatsapp-bridge/whatsapp-bridge": not found`
- **Fix:** Modified Dockerfile to build Go binary during Docker build
- **Commit:** `e7c6033` - Should fix all deployment failures

---

## ⚙️ GO VERSION MISMATCH FIX (14:57 UTC)

**NEW BUILD ERROR:** Go version compatibility issue
```
go: downloading go1.23 (linux/amd64) 
go: download go1.23 for linux/amd64: toolchain not available
```

**PROBLEM:** 
- Dockerfile installed Go 1.21.5
- `go.mod` requires Go 1.23
- Go 1.21.5 couldn't download Go 1.23 toolchain

**FIX:** Updated Dockerfile to install Go 1.23.0
- **Commit:** `087271c`

---

**Last Updated:** 15 Oct 2025, 14:57 UTC  
**Status:** Go version fixed - Build should now succeed
