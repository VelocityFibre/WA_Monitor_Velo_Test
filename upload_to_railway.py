#!/usr/bin/env python3
"""
Automatically upload WhatsApp session data chunks to Railway environment variables
"""
import os
import subprocess
import sys
from pathlib import Path

def run_command(command, check=True):
    """Run a shell command and return the result"""
    try:
        result = subprocess.run(command, shell=True, capture_output=True, text=True, check=check)
        return result.stdout.strip(), result.stderr.strip()
    except subprocess.CalledProcessError as e:
        print(f"❌ Error running command: {command}")
        print(f"   Error: {e.stderr}")
        return None, e.stderr

def check_railway_cli():
    """Check if Railway CLI is installed"""
    stdout, stderr = run_command("railway --version", check=False)
    if stdout is None:
        print("❌ Railway CLI is not installed!")
        print("📥 Install it with: npm install -g @railway/cli")
        print("🔗 Or visit: https://docs.railway.app/develop/cli")
        return False
    print(f"✅ Railway CLI found: {stdout}")
    return True

def check_railway_login():
    """Check if user is logged into Railway"""
    stdout, stderr = run_command("railway whoami", check=False)
    if stdout is None or "not logged in" in stderr.lower():
        print("❌ Not logged into Railway!")
        print("🔑 Login with: railway login")
        return False
    print(f"✅ Logged in as: {stdout}")
    return True

def get_railway_projects():
    """Get list of Railway projects"""
    stdout, stderr = run_command("railway projects", check=False)
    if stdout is None:
        print("❌ Could not fetch Railway projects")
        return []
    return stdout

def select_project():
    """Let user select or confirm Railway project"""
    projects = get_railway_projects()
    print("\n📋 Available Railway projects:")
    print(projects)
    
    # Check if already linked to a project
    stdout, stderr = run_command("railway status", check=False)
    if stdout and "Project:" in stdout:
        current_project = [line for line in stdout.split('\n') if 'Project:' in line][0]
        print(f"\n🔗 Currently linked to: {current_project}")
        
        confirm = input("\nUse this project? (y/n): ").lower().strip()
        if confirm == 'y':
            return True
    
    # Ask user to link to correct project
    print("\n🔧 Please link to your WA_Monitor_Velo_Test project:")
    print("   Run: railway link")
    print("   Then run this script again")
    return False

def read_session_data():
    """Read WhatsApp session data from file"""
    session_file = Path("whatsapp-session-package.b64")
    if not session_file.exists():
        print(f"❌ Session file not found: {session_file}")
        return None
    
    try:
        with open(session_file, 'r') as f:
            data = f.read().strip()
        print(f"✅ Read session data: {len(data):,} characters")
        return data
    except Exception as e:
        print(f"❌ Error reading session file: {e}")
        return None

def split_session_data(data, chunk_size=32000):
    """Split session data into chunks"""
    chunks = []
    for i in range(0, len(data), chunk_size):
        chunks.append(data[i:i + chunk_size])
    
    print(f"✅ Split into {len(chunks)} chunks (max {chunk_size} chars each)")
    return chunks

def upload_chunks(chunks):
    """Upload chunks to Railway as environment variables"""
    print(f"\n🚀 Starting upload of {len(chunks)} environment variables...")
    
    successful_uploads = 0
    failed_uploads = []
    
    for i, chunk in enumerate(chunks, 1):
        var_name = f"WHATSAPP_SESSION_DATA_{i}"
        
        # Escape the chunk data for shell command
        # Use single quotes and escape any single quotes in the data
        escaped_chunk = chunk.replace("'", "'\"'\"'")
        
        print(f"📤 Uploading {var_name}... ", end="", flush=True)
        
        command = f"railway variables set {var_name}='{escaped_chunk}'"
        stdout, stderr = run_command(command, check=False)
        
        if stdout is not None:
            print("✅")
            successful_uploads += 1
        else:
            print("❌")
            failed_uploads.append((var_name, stderr))
        
        # Add a small delay to avoid rate limiting
        import time
        time.sleep(0.1)
    
    print(f"\n📊 Upload Summary:")
    print(f"   ✅ Successful: {successful_uploads}")
    print(f"   ❌ Failed: {len(failed_uploads)}")
    
    if failed_uploads:
        print(f"\n❌ Failed uploads:")
        for var_name, error in failed_uploads:
            print(f"   {var_name}: {error}")
        return False
    
    return True

def main():
    print("🚂 Railway WhatsApp Session Uploader")
    print("=" * 50)
    
    # Check prerequisites
    if not check_railway_cli():
        sys.exit(1)
    
    if not check_railway_login():
        sys.exit(1)
    
    if not select_project():
        sys.exit(1)
    
    # Read and process session data
    session_data = read_session_data()
    if not session_data:
        sys.exit(1)
    
    chunks = split_session_data(session_data)
    
    # Confirm before upload
    print(f"\n⚠️  About to upload {len(chunks)} environment variables to Railway")
    confirm = input("Continue? (y/n): ").lower().strip()
    
    if confirm != 'y':
        print("❌ Upload cancelled")
        sys.exit(0)
    
    # Upload chunks
    success = upload_chunks(chunks)
    
    if success:
        print("\n🎉 All chunks uploaded successfully!")
        print("\n🔄 Your Railway deployment will now automatically combine these")
        print("   variables when it starts up.")
        print("\n💡 You can now deploy or restart your Railway service.")
    else:
        print("\n❌ Some uploads failed. Please check the errors above.")
        sys.exit(1)

if __name__ == "__main__":
    main()