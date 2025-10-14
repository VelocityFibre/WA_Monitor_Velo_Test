#!/bin/bash

# Velo Test - Environment Setup Script
# This script sets up the complete environment for Velo Test

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project directory
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"

echo -e "${BLUE}🔧 Velo Test Environment Setup${NC}"
echo -e "${BLUE}Project Directory: $PROJECT_DIR${NC}"
echo ""

# Function to check command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to print step header
print_step() {
    echo ""
    echo -e "${BLUE}📋 Step $1: $2${NC}"
    echo -e "${BLUE}$(printf '=%.0s' {1..50})${NC}"
}

# Function to create directory if it doesn't exist
ensure_directory() {
    if [ ! -d "$1" ]; then
        echo -e "${YELLOW}📁 Creating directory: $1${NC}"
        mkdir -p "$1"
    else
        echo -e "${GREEN}✅ Directory exists: $1${NC}"
    fi
}

print_step "1" "Checking System Requirements"

echo -e "${BLUE}🔍 Checking system requirements...${NC}"

# Check operating system
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo -e "${GREEN}✅ Operating System: Linux${NC}"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    echo -e "${GREEN}✅ Operating System: macOS${NC}"
else
    echo -e "${YELLOW}⚠️  Operating System: $OSTYPE (should work, but tested on Linux/macOS)${NC}"
fi

# Check RAM
if command_exists free; then
    total_ram=$(free -m | awk 'NR==2{printf "%.0f", $2/1024}')
    if [ "$total_ram" -ge 4 ]; then
        echo -e "${GREEN}✅ RAM: ${total_ram}GB (minimum 4GB required)${NC}"
    else
        echo -e "${RED}❌ RAM: ${total_ram}GB (minimum 4GB required)${NC}"
        exit 1
    fi
elif command_exists sysctl; then
    total_ram=$(sysctl -n hw.memsize | awk '{printf "%.0f", $1/1024/1024/1024}')
    if [ "$total_ram" -ge 4 ]; then
        echo -e "${GREEN}✅ RAM: ${total_ram}GB (minimum 4GB required)${NC}"
    else
        echo -e "${RED}❌ RAM: ${total_ram}GB (minimum 4GB required)${NC}"
        exit 1
    fi
fi

# Check available disk space
available_space=$(df -BG . | awk 'NR==2 {print $4}' | sed 's/G//')
if [ "$available_space" -ge 2 ]; then
    echo -e "${GREEN}✅ Disk Space: ${available_space}GB available (minimum 2GB required)${NC}"
else
    echo -e "${RED}❌ Disk Space: ${available_space}GB available (minimum 2GB required)${NC}"
    exit 1
fi

print_step "2" "Installing System Dependencies"

echo -e "${BLUE}🔍 Checking and installing system dependencies...${NC}"

# Check Python 3.11+
if command_exists python3.11; then
    PYTHON_CMD="python3.11"
    echo -e "${GREEN}✅ Python 3.11+ found: $(python3.11 --version)${NC}"
elif command_exists python3; then
    python_version=$(python3 --version 2>&1 | grep -oP '\d+\.\d+' | head -1)
    major=$(echo $python_version | cut -d. -f1)
    minor=$(echo $python_version | cut -d. -f2)

    if [ "$major" -eq 3 ] && [ "$minor" -ge 11 ]; then
        PYTHON_CMD="python3"
        echo -e "${GREEN}✅ Python 3.11+ found: $(python3 --version)${NC}"
    else
        echo -e "${YELLOW}⚠️  Python 3.11+ not found. Current: $(python3 --version)${NC}"
        echo -e "${YELLOW}   Please install Python 3.11 or later${NC}"
        echo -e "${YELLOW}   Ubuntu/Debian: sudo apt install python3.11 python3.11-pip python3.11-venv${NC}"
        echo -e "${YELLOW}   macOS: brew install python@3.11${NC}"
        exit 1
    fi
else
    echo -e "${RED}❌ Python 3 not found${NC}"
    echo -e "${RED}   Please install Python 3.11 or later${NC}"
    exit 1
fi

# Check Go
if command_exists go; then
    go_version=$(go version 2>&1)
    echo -e "${GREEN}✅ Go found: $go_version${NC}"
else
    echo -e "${RED}❌ Go not found${NC}"
    echo -e "${RED}   Please install Go 1.19 or later${NC}"
    echo -e "${RED}   Ubuntu/Debian: sudo apt install golang-go${NC}"
    echo -e "${RED}   macOS: brew install go${NC}"
    echo -e "${RED}   Download: https://golang.org/dl/${NC}"
    exit 1
fi

# Check Git
if command_exists git; then
    echo -e "${GREEN}✅ Git found: $(git --version)${NC}"
else
    echo -e "${RED}❌ Git not found${NC}"
    echo -e "${RED}   Please install Git${NC}"
    exit 1
fi

# Check UV (optional but recommended)
if command_exists uv; then
    echo -e "${GREEN}✅ UV found: $(uv --version)${NC}"
    USE_UV=true
else
    echo -e "${YELLOW}⚠️  UV not found. Will use pip instead${NC}"
    echo -e "${YELLOW}   Recommended: Install UV for faster package management${NC}"
    echo -e "${YELLOW}   Install: curl -LsSf https://astral.sh/uv/install.sh | sh${NC}"
    USE_UV=false
fi

print_step "3" "Setting Up Project Structure"

echo -e "${BLUE}📁 Creating project directories...${NC}"

ensure_directory "services"
ensure_directory "scripts"
ensure_directory "config"
ensure_directory "docs"
ensure_directory "logs"
ensure_directory "docker-data/whatsapp-sessions"
ensure_directory "docker-data/bridge-logs"
ensure_directory "docker-data/monitor-logs"

print_step "4" "Setting Up Python Environment"

echo -e "${BLUE}🐍 Setting up Python virtual environment...${NC}"

# Create virtual environment if it doesn't exist
if [ ! -d ".venv" ]; then
    echo -e "${YELLOW}📦 Creating Python virtual environment...${NC}"
    $PYTHON_CMD -m venv .venv
    echo -e "${GREEN}✅ Virtual environment created${NC}"
else
    echo -e "${GREEN}✅ Virtual environment already exists${NC}"
fi

# Activate virtual environment
echo -e "${YELLOW}🔄 Activating virtual environment...${NC}"
source .venv/bin/activate

# Upgrade pip
echo -e "${YELLOW}⬆️  Upgrading pip...${NC}"
pip install --upgrade pip

# Install Python dependencies
if [ -f "services/requirements.txt" ]; then
    echo -e "${YELLOW}📦 Installing Python dependencies...${NC}"
    if [ "$USE_UV" = true ]; then
        uv pip install -r services/requirements.txt
    else
        pip install -r services/requirements.txt
    fi
    echo -e "${GREEN}✅ Python dependencies installed${NC}"
else
    echo -e "${RED}❌ services/requirements.txt not found${NC}"
    exit 1
fi

print_step "5" "Setting Up Go Environment"

echo -e "${BLUE}🐹 Setting up Go dependencies...${NC}"

if [ -d "services/whatsapp-bridge" ]; then
    cd services/whatsapp-bridge

    echo -e "${YELLOW}📦 Downloading Go modules...${NC}"
    go mod download
    go mod tidy

    # Test build
    echo -e "${YELLOW}🔨 Testing Go build...${NC}"
    go build -o whatsapp-bridge main.go

    if [ -f "whatsapp-bridge" ]; then
        echo -e "${GREEN}✅ Go build successful${NC}"
        rm -f whatsapp-bridge  # Clean up test build
    else
        echo -e "${RED}❌ Go build failed${NC}"
        exit 1
    fi

    cd ../..
else
    echo -e "${RED}❌ services/whatsapp-bridge directory not found${NC}"
    exit 1
fi

print_step "6" "Setting Up Environment Configuration"

echo -e "${BLUE}⚙️  Setting up environment configuration...${NC}"

# Copy .env template if .env doesn't exist
if [ ! -f ".env" ]; then
    if [ -f ".env.template" ]; then
        echo -e "${YELLOW}📋 Copying environment template...${NC}"
        cp .env.template .env
        echo -e "${GREEN}✅ .env file created from template${NC}"
        echo -e "${YELLOW}⚠️  Please edit .env file with your actual credentials${NC}"
        echo -e "${YELLOW}   Required: NEON_DATABASE_URL, GOOGLE_SHEETS_CREDENTIALS_PATH, GOOGLE_SHEETS_ID, LLM_API_KEY${NC}"
    else
        echo -e "${RED}❌ .env.template not found${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✅ .env file already exists${NC}"
fi

print_step "7" "Setting Up Service Scripts"

echo -e "${BLUE}🔧 Making scripts executable...${NC}"

# Make all shell scripts executable
find scripts/ -name "*.sh" -exec chmod +x {} \;
echo -e "${GREEN}✅ Scripts made executable${NC}"

# Make Python scripts executable
find services/ -name "*.py" -exec chmod +x {} \;
echo -e "${GREEN}✅ Python scripts made executable${NC}"

print_step "8" "Creating Helper Scripts"

echo -e "${BLUE}📝 Creating helper scripts...${NC}"

# Create quick start script
cat > quick_start.sh << 'EOF'
#!/bin/bash
# Quick Start Script for Velo Test

echo "🚀 Starting Velo Test Quick Start..."

# Start services
./scripts/start_all.sh

# Show status
echo ""
echo "📊 Service Status:"
./scripts/health_check.sh

echo ""
echo "🎉 Velo Test is ready!"
echo "📋 Next steps:"
echo "   1. Scan QR code for WhatsApp (if prompted)"
echo "   2. Test by posting 'DR9999999' in your Velo Test WhatsApp group"
echo "   3. Check logs: tail -f logs/drop_monitor.log"
echo "   4. Stop services: ./scripts/stop_all.sh"
EOF

chmod +x quick_start.sh

# Create development script
cat > dev_start.sh << 'EOF'
#!/bin/bash
# Development Start Script for Velo Test

echo "🛠️  Starting Velo Test in Development Mode..."

# Set debug mode
export DEBUG_MODE=true
export LOG_LEVEL=DEBUG

# Start services with debug logging
./scripts/start_all.sh

echo ""
echo "🐛 Development mode active!"
echo "📋 Debug commands:"
echo "   - View all logs: tail -f logs/*.log"
echo "   - Health check: ./scripts/health_check.sh"
echo "   - Stop services: ./scripts/stop_all.sh"
EOF

chmod +x dev_start.sh

echo -e "${GREEN}✅ Helper scripts created${NC}"

print_step "9" "Running Initial Health Check"

echo -e "${BLUE}🏥 Running initial health check...${NC}"

# Run health check to verify setup
if ./scripts/health_check.sh; then
    echo -e "${GREEN}✅ Health check passed!${NC}"
else
    echo -e "${YELLOW}⚠️  Health check completed with warnings${NC}"
    echo -e "${YELLOW}   Some services may need configuration before starting${NC}"
fi

print_step "10" "Final Setup Summary"

echo ""
echo -e "${GREEN}🎉 Velo Test environment setup complete!${NC}"
echo ""
echo -e "${BLUE}📋 Setup Summary:${NC}"
echo -e "${GREEN}✅ Project structure created${NC}"
echo -e "${GREEN}✅ Python environment configured${NC}"
echo -e "${GREEN}✅ Go dependencies installed${NC}"
echo -e "${GREEN}✅ Service scripts created${NC}"
echo -e "${GREEN}✅ Helper scripts ready${NC}"
echo ""
echo -e "${BLUE}🚀 Quick Start Commands:${NC}"
echo -e "${BLUE}   • Quick start: ./quick_start.sh${NC}"
echo -e "${BLUE}   • Development mode: ./dev_start.sh${NC}"
echo -e "${BLUE}   • Manual start: ./scripts/start_all.sh${NC}"
echo -e "${BLUE}   • Health check: ./scripts/health_check.sh${NC}"
echo -e "${BLUE}   • Stop services: ./scripts/stop_all.sh${NC}"
echo ""
echo -e "${YELLOW}⚠️  Important Next Steps:${NC}"
echo -e "${YELLOW}   1. Edit .env file with your actual credentials${NC}"
echo -e "${YELLOW}   2. Set up Google Sheets API and credentials${NC}"
echo -e "${YELLOW}   3. Configure Neon PostgreSQL database${NC}"
echo -e "${YELLOW}   4. Get OpenRouter API key for AI functionality${NC}"
echo -e "${YELLOW}   5. Test the complete workflow${NC}"
echo ""
echo -e "${BLUE}📚 Documentation:${NC}"
echo -e "${BLUE}   • Operations guide: claude.md${NC}"
echo -e "${BLUE}   • Deployment guide: README_DEPLOYMENT.md${NC}"
echo ""
echo -e "${GREEN}✨ Ready to deploy Velo Test! ✨${NC}"