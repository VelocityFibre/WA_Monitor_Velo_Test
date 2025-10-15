#!/usr/bin/env python3
"""
Improved WhatsApp session data uploader for Railway - uses temporary files to avoid command length limits
"""
import os
import subprocess
import sys
import tempfile
from pathlib import Path

def run_command(command, check=True, input_data=None):
    """Run a shell command and return the result"""
    try:
        result = subprocess.run(
            command, 
            shell=True, 
            capture_output=True, 
            text=True, 
            check=check,
            input=input_data
        )
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

def check_project_status():
    """Check Railway project status"""
    stdout, stderr = run_command("railway status", check=False)
    if stdout and "Project:" in stdout:
        print(f"✅ Connected to Railway project")
        for line in stdout.split('\n'):
            if line.strip():
                print(f"   {line}")
        return True
    else:
        print("❌ Not linked to a Railway project")
        print("🔧 Run: railway link")
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

def upload_chunks_via_file(chunks):
    """Upload chunks using temporary files to avoid shell command limits"""
    print(f"\n🚀 Starting upload of {len(chunks)} environment variables...")
    
    successful_uploads = 0
    failed_uploads = []
    
    for i, chunk in enumerate(chunks, 1):
        var_name = f"WHATSAPP_SESSION_DATA_{i}"
        
        print(f"📤 Uploading {var_name}... ", end="", flush=True)
        
        # Create temporary file with the chunk data
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as temp_file:
            temp_file.write(chunk)
            temp_path = temp_file.name
        
        try:
            # Use the railway CLI with file input
            command = f'railway variables --set "{var_name}=$(cat {temp_path})"'
            stdout, stderr = run_command(command, check=False)
            
            if stdout is not None:
                print("✅")
                successful_uploads += 1
            else:
                print("❌")
                failed_uploads.append((var_name, stderr))
        
        finally:
            # Clean up temporary file
            os.unlink(temp_path)
        
        # Small delay to avoid rate limiting
        import time
        time.sleep(0.05)
    
    print(f"\n📊 Upload Summary:")
    print(f"   ✅ Successful: {successful_uploads}")
    print(f"   ❌ Failed: {len(failed_uploads)}")
    
    if failed_uploads:
        print(f"\n❌ Failed uploads:")
        for var_name, error in failed_uploads[:5]:  # Show first 5 errors
            print(f"   {var_name}: {error}")
        if len(failed_uploads) > 5:
            print(f"   ... and {len(failed_uploads) - 5} more")
        return False
    
    return True

def verify_upload(chunk_count):
    """Verify that variables were uploaded correctly"""
    print("\n🔍 Verifying upload...")
    
    # Get current variables
    stdout, stderr = run_command("railway variables --json", check=False)
    if stdout is None:
        print("❌ Could not fetch variables for verification")
        return False
    
    try:
        import json
        variables = json.loads(stdout)
        
        # Count WhatsApp session variables
        session_vars = [key for key in variables.keys() if key.startswith("WHATSAPP_SESSION_DATA_")]
        
        print(f"✅ Found {len(session_vars)} WhatsApp session variables")
        
        if len(session_vars) == chunk_count:
            print("🎉 All variables verified successfully!")
            return True
        else:
            print(f"⚠️  Expected {chunk_count} variables, found {len(session_vars)}")
            return False
            
    except json.JSONDecodeError:
        print("❌ Could not parse variables JSON")
        return False

def main():
    print("🚂 Railway WhatsApp Session Uploader v2")
    print("=" * 50)
    
    # Check prerequisites
    if not check_railway_cli():
        sys.exit(1)
    
    if not check_railway_login():
        sys.exit(1)
    
    if not check_project_status():
        sys.exit(1)
    
    # Read and process session data
    session_data = read_session_data()
    if not session_data:
        sys.exit(1)
    
    chunks = split_session_data(session_data)
    
    # Confirm before upload
    print(f"\n⚠️  About to upload {len(chunks)} environment variables to Railway")
    print("   This will use temporary files to handle the large data safely.")
    confirm = input("Continue? (y/n): ").lower().strip()
    
    if confirm != 'y':
        print("❌ Upload cancelled")
        sys.exit(0)
    
    # Upload chunks
    success = upload_chunks_via_file(chunks)
    
    if success:
        # Verify the upload
        if verify_upload(len(chunks)):
            print("\n🎉 Upload completed and verified successfully!")
            print("\n🔄 Your Railway deployment will now automatically combine these")
            print("   variables when it starts up.")
            print("\n💡 You can now deploy or restart your Railway service.")
        else:
            print("\n⚠️  Upload may have issues. Check Railway dashboard manually.")
    else:
        print("\n❌ Upload failed. Please check the errors above.")
        sys.exit(1)

if __name__ == "__main__":
    main()