# Railway Deployment Guide - Velo Test WhatsApp Monitor

## 🚀 Why Railway is Perfect for This Project

✅ **Zero infrastructure management**  
✅ **Automatic Docker builds**  
✅ **Built-in persistent volumes** for WhatsApp sessions  
✅ **Environment variables management**  
✅ **Automatic HTTPS and domains**  
✅ **Pay-per-use pricing** (~$5-15/month)  
✅ **Built-in monitoring and logs**  

## Quick Deployment (5 Minutes)

### Step 1: Install Railway CLI
```bash
npm install -g @railway/cli
# or
curl -fsSL https://railway.app/install.sh | sh
```

### Step 2: Login to Railway
```bash
railway login
```

### Step 3: Initialize Project
```bash
cd /home/louisdup/VF/deployments/WA_monitor\ _Velo_Test
railway init
# Select: "Deploy from GitHub" or "Deploy from local directory"
```

### Step 4: Set Environment Variables
```bash
# Set all required environment variables
railway variables set NEON_DATABASE_URL="postgresql://neondb_owner:npg_RIgDxzo4St6d@ep-damp-credit-a857vku0-pooler.eastus2.azure.neon.tech/neondb?sslmode=require&channel_binding=require"

railway variables set GOOGLE_SHEETS_ID="1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"

railway variables set VELO_TEST_GROUP_JID="120363421664266245@g.us"

railway variables set LOG_LEVEL="INFO"

railway variables set DEBUG_MODE="false"

# Optional: LLM API for advanced features
railway variables set LLM_API_KEY="your-api-key"
```

### Step 5: Upload Google Credentials
```bash
# Railway will need access to credentials.json
# You can either:
# 1. Include it in your repo (not recommended for security)
# 2. Set it as a base64 environment variable (recommended)

# Option 2: Base64 encode credentials
CREDS_B64=$(base64 -w 0 credentials.json)
railway variables set GOOGLE_APPLICATION_CREDENTIALS_B64="$CREDS_B64"
```

### Step 6: Deploy
```bash
# Deploy using the Railway-optimized compose file
railway up --dockerfile docker-compose.railway.yml
```

## Environment Variables for Railway

### Required Variables:
```bash
NEON_DATABASE_URL=postgresql://neondb_owner:...
GOOGLE_SHEETS_ID=1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk
VELO_TEST_GROUP_JID=120363421664266245@g.us
GOOGLE_APPLICATION_CREDENTIALS_B64=base64_encoded_credentials
LOG_LEVEL=INFO
DEBUG_MODE=false
```

### Optional Variables:
```bash
LLM_API_KEY=your_openai_or_anthropic_key
LLM_MODEL=x.ai/grok-2-1212:free
FEEDBACK_COOLDOWN=300
```

## Railway-Specific Modifications

### 1. Handle Google Credentials
Since Railway doesn't easily support file uploads, we'll modify the services to handle base64 credentials:

```bash
# Add to your Docker containers
echo "$GOOGLE_APPLICATION_CREDENTIALS_B64" | base64 -d > /app/credentials.json
```

### 2. Port Configuration  
Railway automatically assigns a PORT environment variable:
```yaml
ports:
  - "${PORT:-8080}:8080"
```

### 3. Persistent Volumes
Railway supports Docker volumes for WhatsApp session persistence:
```yaml
volumes:
  whatsapp-sessions:
    driver: local
    name: velo-whatsapp-sessions
```

## Deployment Status Monitoring

### Check Deployment Status
```bash
# View deployment logs
railway logs

# Check service status
railway status

# View environment variables
railway variables
```

### Access Your Application
```bash
# Get your app URL
railway domain
# Example: https://your-app-name.railway.app
```

## Cost Estimation

### Railway Pricing (Pay-per-use):
- **Starter**: $5/month base + usage
- **Expected cost**: $10-20/month for 24/7 operation
- **What you get**:
  - Persistent storage
  - Automatic scaling
  - SSL certificates
  - Monitoring & logs
  - 99.9% uptime SLA

## Backup Strategy on Railway

### 1. WhatsApp Sessions Backup
Railway persistent volumes are automatically backed up, but you can also:

```bash
# Create periodic backups of session data
railway run "tar -czf backup-$(date +%Y%m%d).tar.gz /app/store && curl -X POST -F 'file=@backup-$(date +%Y%m%d).tar.gz' YOUR_BACKUP_WEBHOOK"
```

### 2. Configuration Backup
Your entire configuration is in code, so git commits are your backup.

## Troubleshooting Railway Deployment

### Common Issues:

1. **Build Failures**:
   ```bash
   railway logs --deployment
   ```

2. **Environment Variable Issues**:
   ```bash
   railway variables
   railway variables set VAR_NAME="value"
   ```

3. **Service Health Checks**:
   ```bash
   railway logs --service whatsapp-bridge
   ```

4. **Persistent Volume Issues**:
   ```bash
   railway logs | grep -i volume
   ```

## Migration from Railway (if needed)

If you ever need to move to another platform:

1. **Export WhatsApp Sessions**:
   ```bash
   railway run "tar -czf whatsapp-backup.tar.gz /app/store"
   ```

2. **Export Environment Variables**:
   ```bash
   railway variables > railway-vars.txt
   ```

3. **Use existing docker-compose.yml** for any other platform

## Advantages of Railway for Your Use Case

1. **Perfect for long-running services** like WhatsApp monitors
2. **Automatic restarts** if services crash
3. **Built-in monitoring** - see exactly when drops are processed
4. **Easy scaling** if you add more WhatsApp groups
5. **Zero server maintenance** - focus on your business logic
6. **Integrated secrets management** for API keys

---

## Ready to Deploy? 

**Execute these commands:**

```bash
# Install Railway CLI
npm install -g @railway/cli

# Login
railway login

# Initialize and deploy
cd /home/louisdup/VF/deployments/WA_monitor\ _Velo_Test
railway init
railway up

# Set environment variables (see above)
# Monitor deployment
railway logs -f
```

**Your WhatsApp monitor will be live in ~5 minutes!** 🎉

---

**Railway vs DigitalOcean Summary:**
- **Railway**: Perfect for quick deployment, low maintenance, predictable costs
- **DigitalOcean**: Better for enterprise, high-traffic, full control needs

**Recommendation**: Start with Railway, migrate to DigitalOcean if you need more control or lower costs at scale.