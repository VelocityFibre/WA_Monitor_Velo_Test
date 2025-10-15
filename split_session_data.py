#!/usr/bin/env python3
"""
Split WhatsApp Session Data for Railway Environment Variables

Railway has a 32KB limit per environment variable. This script takes your 
base64-encoded WhatsApp session data and splits it into chunks that fit 
within this limit.

Usage:
    python3 split_session_data.py <base64_session_data>

The script will output the chunks and instructions for Railway deployment.
"""

import sys
import os
import math

def split_session_data(base64_data):
    """Split base64 data into chunks that fit Railway's 32KB limit"""
    
    # Railway limit is 32768 characters, but let's use 32000 to be safe
    CHUNK_SIZE = 32000
    
    # Remove any whitespace/newlines from the data
    clean_data = base64_data.strip().replace('\n', '').replace('\r', '').replace(' ', '')
    
    # Calculate number of chunks needed
    total_length = len(clean_data)
    num_chunks = math.ceil(total_length / CHUNK_SIZE)
    
    print(f"📊 Session data analysis:")
    print(f"   Total length: {total_length:,} characters")
    print(f"   Number of chunks needed: {num_chunks}")
    print(f"   Chunk size limit: {CHUNK_SIZE:,} characters")
    print("")
    
    # Split into chunks
    chunks = []
    for i in range(num_chunks):
        start = i * CHUNK_SIZE
        end = min((i + 1) * CHUNK_SIZE, total_length)
        chunk = clean_data[start:end]
        chunks.append(chunk)
        print(f"📦 Chunk {i+1}: {len(chunk):,} characters")
    
    return chunks

def print_railway_instructions(chunks):
    """Print instructions for adding chunks to Railway"""
    
    print("\n" + "="*70)
    print("🚂 RAILWAY DEPLOYMENT INSTRUCTIONS")
    print("="*70)
    print("")
    print("1. Go to your Railway project: WA_Monitor_Velo_Test")
    print("2. Click on the 'Variables' tab")
    print("3. Add the following environment variables:")
    print("")
    
    for i, chunk in enumerate(chunks, 1):
        print(f"   Variable name: WHATSAPP_SESSION_DATA_{i}")
        print(f"   Variable value: (Copy the chunk below)")
        print("")
        print(f"   --- CHUNK {i} START ---")
        print(chunk)
        print(f"   --- CHUNK {i} END ---")
        print("")
    
    print("4. After adding all variables, click 'Save' or deploy your service")
    print("5. The startup script will automatically combine all chunks")
    print("")
    print("⚠️  IMPORTANT:")
    print("   - Make sure to copy each chunk EXACTLY as shown")
    print("   - Do not add extra spaces or newlines")
    print("   - Variable names must be exactly: WHATSAPP_SESSION_DATA_1, WHATSAPP_SESSION_DATA_2, etc.")
    print("")
    print("✅ Once deployed, your WhatsApp service should start without requiring a QR code!")

def main():
    # Check if we should read from the local file
    session_file = "whatsapp-session-package.b64"
    
    if len(sys.argv) == 1 and os.path.exists(session_file):
        print(f"Reading session data from {session_file}...")
        with open(session_file, 'r') as f:
            base64_data = f.read().strip()
    elif len(sys.argv) == 2:
        base64_data = sys.argv[1]
    else:
        print("Usage: python3 split_session_data.py [base64_session_data]")
        print("")
        print("Examples:")
        print("  python3 split_session_data.py 'H4sIAAAAAAAAA+xcCVwTE...'")
        print("  python3 split_session_data.py  # reads from whatsapp-session-package.b64")
        print("")
        sys.exit(1)
    
    if not base64_data.startswith('H4sIA'):
        print("⚠️  Warning: The data doesn't appear to start with expected base64 header 'H4sIA'")
        print("   Make sure you're using the complete base64-encoded session data")
        print("")
    
    try:
        chunks = split_session_data(base64_data)
        print_railway_instructions(chunks)
        
    except Exception as e:
        print(f"❌ Error processing session data: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()