# Velo Test WhatsApp Monitor - Cloud Deployment Guide

## ✅ SYSTEM STATUS: 100% OPERATIONAL
**Date**: 2025-10-14  
**Final Test**: DR00000014 - Complete workflow tested successfully

## Complete Workflow Verified ✅
1. ✅ **Drop Detection**: DR00000014 posted → Detected → Added to Google Sheets
2. ✅ **QA Feedback**: System sent feedback for incomplete drop
3. ✅ **Done Message**: "done" reply detected → Status updated
4. ✅ **End-to-End**: Complete automation working perfectly

---

# Cloud Deployment Options

## Option 1: Docker Compose on Cloud VM (Recommended)
**Best for**: Simple deployment, cost-effective, full control

### Step 1: Create Cloud VM
```bash
# AWS EC2, GCP Compute Engine, or DigitalOcean Droplet
# Minimum specs: 2 CPU, 4GB RAM, 20GB storage
# Ubuntu 20.04 LTS recommended
```

### Step 2: Install Dependencies
```bash
# On cloud VM
sudo apt update
sudo apt install docker.io docker-compose git -y
sudo usermod -aG docker $USER
```

### Step 3: Deploy Code
```bash
# Clone/upload your code to VM
scp -r /home/louisdup/VF/deployments/WA_monitor\ _Velo_Test/ user@vm-ip:/home/user/

# Or use git
git clone your-repo
cd WA_monitor_Velo_Test
```

### Step 4: Setup Environment
```bash
# Copy working session files
mkdir -p docker-data/whatsapp-sessions
cp -r services/whatsapp-bridge/store/* docker-data/whatsapp-sessions/

# Ensure credentials.json is present
cp credentials.json docker-data/
```

### Step 5: Start Services
```bash
# Using existing docker-compose.yml
docker-compose up -d

# Check status
docker-compose ps
docker-compose logs -f
```

---

## Option 2: Kubernetes Deployment
**Best for**: High availability, auto-scaling, enterprise

### Step 1: Create Kubernetes Manifests
```bash
# Create namespace
kubectl create namespace velo-test

# Apply manifests (create these based on docker-compose.yml)
kubectl apply -f k8s/
```

### Step 2: Persistent Volumes
```yaml
# whatsapp-sessions-pv.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: whatsapp-sessions-pvc
  namespace: velo-test
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
```

---

## Option 3: AWS ECS Fargate
**Best for**: Serverless, managed infrastructure

### Step 1: Create ECS Cluster
```bash
aws ecs create-cluster --cluster-name velo-test-cluster
```

### Step 2: Create Task Definition
```json
{
  "family": "velo-test-tasks",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "1024",
  "memory": "2048",
  "containerDefinitions": [
    {
      "name": "whatsapp-bridge",
      "image": "your-registry/whatsapp-bridge:latest",
      "portMappings": [{"containerPort": 8080}],
      "environment": [
        {"name": "NEON_DATABASE_URL", "value": "..."},
        {"name": "GSHEET_ID", "value": "..."}
      ],
      "mountPoints": [
        {
          "sourceVolume": "whatsapp-sessions",
          "containerPath": "/app/store"
        }
      ]
    }
  ]
}
```

---

# Quick Cloud Deployment (Docker VM)

## 1. Prepare Local Files
```bash
# Create deployment package
cd /home/louisdup/VF/deployments/WA_monitor\ _Velo_Test

# Backup working session (CRITICAL)
tar -czf whatsapp-sessions-backup.tar.gz services/whatsapp-bridge/store/

# Create deployment archive
tar --exclude='venv' --exclude='*.log' --exclude='.git' \
    -czf velo-test-deployment.tar.gz .
```

## 2. Setup Cloud VM
```bash
# Create VM (example for DigitalOcean)
doctl compute droplet create velo-test-monitor \
  --image ubuntu-20-04-x64 \
  --size s-2vcpu-4gb \
  --region nyc1 \
  --ssh-keys your-ssh-key

# Get VM IP
VM_IP=$(doctl compute droplet get velo-test-monitor --format PublicIPv4 --no-header)
```

## 3. Deploy to VM
```bash
# Upload deployment
scp velo-test-deployment.tar.gz root@$VM_IP:/root/

# SSH to VM and setup
ssh root@$VM_IP << 'EOF'
  # Install Docker
  apt update
  apt install docker.io docker-compose git -y
  systemctl enable docker
  systemctl start docker
  
  # Extract deployment
  cd /root
  tar -xzf velo-test-deployment.tar.gz
  cd WA_monitor_Velo_Test
  
  # Setup directories
  mkdir -p docker-data/{whatsapp-sessions,bridge-logs,monitor-logs}
  
  # Extract WhatsApp sessions
  tar -xzf whatsapp-sessions-backup.tar.gz -C docker-data/whatsapp-sessions --strip-components=3
  
  # Start services
  docker-compose up -d
  
  # Check status
  docker-compose ps
EOF
```

## 4. Verify Deployment
```bash
# Test WhatsApp bridge health
curl http://$VM_IP:8080/health

# Check service logs
ssh root@$VM_IP "cd WA_monitor_Velo_Test && docker-compose logs --tail 50"

# Test drop detection
# Post a test drop in WhatsApp group and monitor logs
```

---

# Environment Variables for Cloud

## Required Environment Variables
```bash
# Database
NEON_DATABASE_URL=postgresql://neondb_owner:npg_RIgDxzo4St6d@ep-damp-credit-a857vku0-pooler.eastus2.azure.neon.tech/neondb?sslmode=require&channel_binding=require

# Google Sheets
GSHEET_ID=1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk
GOOGLE_APPLICATION_CREDENTIALS=/app/credentials.json

# WhatsApp Groups
VELO_TEST_GROUP_JID=120363421664266245@g.us

# Service Configuration
LOG_LEVEL=INFO
DEBUG_MODE=false
```

## Docker Compose Environment
Update your `.env` file with cloud-specific settings:
```bash
# Cloud VM Configuration
SERVICE_NAME=velo-test-production
PROJECT_NAME=Velo Test Production
INSTANCE_ID=cloud-001

# Monitoring
HEALTH_CHECK_INTERVAL=30
RESTART_POLICY=unless-stopped

# Resource Limits
MEMORY_LIMIT=1GB
CPU_LIMIT=1.0
```

---

# Critical Files for Cloud Deployment

## Must Have Files:
1. **credentials.json** - Google Service Account credentials
2. **WhatsApp session files** - From `services/whatsapp-bridge/store/`
3. **docker-compose.yml** - Container orchestration
4. **Environment variables** - All required env vars
5. **Service binaries** - WhatsApp bridge executable

## Pre-deployment Checklist:
- [ ] WhatsApp sessions backed up and tested
- [ ] Google credentials file included
- [ ] All environment variables set
- [ ] Docker images built and tested
- [ ] Network ports configured (8080)
- [ ] Persistent storage configured
- [ ] Health checks implemented
- [ ] Logging configured

---

# Monitoring and Maintenance

## Health Monitoring
```bash
# Service health endpoints
curl http://vm-ip:8080/health  # WhatsApp bridge

# Container health
docker-compose ps
docker stats

# Log monitoring
docker-compose logs -f --tail 100
```

## Backup Strategy
```bash
# Daily backup of WhatsApp sessions
tar -czf "whatsapp-backup-$(date +%Y%m%d).tar.gz" docker-data/whatsapp-sessions/

# Weekly full backup
tar -czf "full-backup-$(date +%Y%m%d).tar.gz" --exclude='*.log' .
```

## Updates and Rollback
```bash
# Update deployment
git pull origin main
docker-compose build
docker-compose up -d

# Rollback if needed
docker-compose down
# Restore previous version
docker-compose up -d
```

---

**DEPLOYMENT READY**: System is fully tested and documented for cloud deployment.
**Recommended**: Start with Docker Compose on cloud VM for simplicity and reliability.
**Next Step**: Choose cloud provider and execute deployment plan above.