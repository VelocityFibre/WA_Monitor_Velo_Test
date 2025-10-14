# Railway + GitHub Deployment Guide

## 🚀 Secure Deployment to Railway via GitHub

### Step 1: Push Code to GitHub (WITHOUT Credentials)
```bash
git add .
git commit -m "Railway deployment configuration - secure"
git push origin master
```

### Step 2: Create Railway Service from GitHub
1. Go to [Railway.app](https://railway.app)
2. Click **"New Project"**
3. Select **"Deploy from GitHub repo"**
4. Choose your repository: `VelocityFibre/WA_Monitor_Velo_Test`
5. Railway will automatically detect the Dockerfile

### Step 3: Set Environment Variables in Railway Dashboard

**CRITICAL:** Add these environment variables in Railway dashboard:

#### Required Variables:
```bash
NEON_DATABASE_URL=postgresql://neondb_owner:npg_RIgDxzo4St6d@ep-damp-credit-a857vku0-pooler.eastus2.azure.neon.tech/neondb?sslmode=require&channel_binding=require

GOOGLE_SHEETS_ID=1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk

VELO_TEST_GROUP_JID=120363421664266245@g.us

GOOGLE_CREDENTIALS_JSON={"type":"service_account","project_id":"sheets-api-473708",...}

LOG_LEVEL=INFO
DEBUG_MODE=false
PROJECT_NAME=Velo Test
SERVICE_NAME=velo-test-production
```

#### How to Set GOOGLE_CREDENTIALS_JSON:
1. Open your local `credentials.json` file
2. Copy the ENTIRE JSON content (as one line)
3. Paste it as the value for `GOOGLE_CREDENTIALS_JSON` in Railway

### Step 4: Deploy and Monitor
1. Railway will automatically build from your GitHub repo
2. Monitor deployment logs in Railway dashboard
3. Check service health at your Railway URL

## Security Benefits ✅

- ✅ **No credentials in GitHub** - Google API keys are safe
- ✅ **No WhatsApp sessions in repo** - Auth tokens protected  
- ✅ **Environment-based config** - Easy to manage secrets
- ✅ **Automatic builds** - Push to GitHub → Deploy to Railway
- ✅ **No file size limits** - GitHub handles the code, Railway builds it

## Railway Environment Variables Setup

### In Railway Dashboard:
1. Go to your project
2. Click **"Variables"** tab
3. Add each variable:

| Variable Name | Value | Notes |
|---------------|--------|-------|
| `NEON_DATABASE_URL` | `postgresql://...` | Database connection |
| `GOOGLE_SHEETS_ID` | `1TYxDLy...` | Google Sheets ID |
| `VELO_TEST_GROUP_JID` | `120363421664266245@g.us` | WhatsApp group |
| `GOOGLE_CREDENTIALS_JSON` | `{"type":"service_account"...}` | Full JSON as string |
| `LOG_LEVEL` | `INFO` | Logging level |
| `DEBUG_MODE` | `false` | Debug flag |

### GOOGLE_CREDENTIALS_JSON Format:
The JSON should look like this (as ONE line):
```json
{"type":"service_account","project_id":"sheets-api-473708","private_key_id":"...","private_key":"-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n","client_email":"...","client_id":"...","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token",...}
```

## Deployment Flow

1. **Code Changes** → Push to GitHub
2. **GitHub** → Triggers Railway deployment  
3. **Railway** → Builds Docker image with environment variables
4. **Service Starts** → All 3 services (Bridge, Monitor, QA) running
5. **WhatsApp Monitor** → Ready to detect drop numbers!

## Monitoring Your Deployment

### Check Service Health:
- Railway dashboard shows service status
- View logs for each service
- Monitor resource usage

### Test the System:
1. Post a drop number in Velo Test WhatsApp group
2. Check Railway logs to see detection
3. Verify Google Sheets update
4. Confirm QA feedback sent

---

**Ready to Deploy? Follow steps 1-4 above!** 🚀