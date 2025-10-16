#!/usr/bin/env python3
"""
WhatsApp Session Persistence Manager
Backs up and restores WhatsApp Bridge session files to/from PostgreSQL database
Works around Railway's volume persistence issues
"""

import os
import sys
import base64
import json
import time
import subprocess
from pathlib import Path
from typing import Optional, Dict, Any
import sqlite3
import psycopg2
from psycopg2.extras import RealDictCursor

class SessionPersistenceManager:
    def __init__(self):
        self.session_dir = Path("./store")
        self.whatsapp_bridge_dir = Path("./services/whatsapp-bridge")
        self.db_url = os.getenv("DATABASE_URL")
        self.project_name = os.getenv("RAILWAY_SERVICE_NAME", "wa_monitor_velo_test")
        
        # Ensure directories exist
        self.session_dir.mkdir(exist_ok=True)
        self.whatsapp_bridge_dir.mkdir(exist_ok=True)
        
    def get_db_connection(self):
        """Get PostgreSQL database connection"""
        if not self.db_url:
            print("❌ DATABASE_URL not found - cannot persist sessions")
            return None
            
        try:
            conn = psycopg2.connect(self.db_url)
            return conn
        except Exception as e:
            print(f"❌ Failed to connect to database: {e}")
            return None
    
    def init_session_table(self) -> bool:
        """Initialize session storage table if it doesn't exist"""
        conn = self.get_db_connection()
        if not conn:
            return False
            
        try:
            with conn.cursor() as cur:
                cur.execute("""
                    CREATE TABLE IF NOT EXISTS whatsapp_sessions (
                        id SERIAL PRIMARY KEY,
                        project_name VARCHAR(255) NOT NULL,
                        file_name VARCHAR(255) NOT NULL,
                        file_data BYTEA NOT NULL,
                        file_size BIGINT NOT NULL,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                        UNIQUE(project_name, file_name)
                    )
                """)
                
                # Create index for faster lookups
                cur.execute("""
                    CREATE INDEX IF NOT EXISTS idx_whatsapp_sessions_project 
                    ON whatsapp_sessions(project_name)
                """)
                
                conn.commit()
                print("✅ Session persistence table initialized")
                return True
        except Exception as e:
            print(f"❌ Failed to initialize session table: {e}")
            return False
        finally:
            conn.close()
    
    def backup_session_files(self) -> bool:
        """Backup all WhatsApp session files to database"""
        conn = self.get_db_connection()
        if not conn:
            return False
            
        try:
            # Find session files to backup
            session_files = []
            
            # Check for session files in various locations
            for pattern in ["*.db", "*.json", "*.key", "*.crt", "session*"]:
                session_files.extend(self.session_dir.glob(pattern))
                session_files.extend(self.whatsapp_bridge_dir.glob(pattern))
            
            if not session_files:
                print("ℹ️  No session files found to backup")
                return True
                
            backup_count = 0
            with conn.cursor() as cur:
                for file_path in session_files:
                    if not file_path.is_file():
                        continue
                        
                    try:
                        # Read file data
                        file_data = file_path.read_bytes()
                        file_name = file_path.name
                        file_size = len(file_data)
                        
                        # Upsert file data
                        cur.execute("""
                            INSERT INTO whatsapp_sessions (project_name, file_name, file_data, file_size, updated_at)
                            VALUES (%s, %s, %s, %s, CURRENT_TIMESTAMP)
                            ON CONFLICT (project_name, file_name)
                            DO UPDATE SET 
                                file_data = EXCLUDED.file_data,
                                file_size = EXCLUDED.file_size,
                                updated_at = CURRENT_TIMESTAMP
                        """, (self.project_name, file_name, file_data, file_size))
                        
                        backup_count += 1
                        print(f"💾 Backed up: {file_name} ({file_size} bytes)")
                        
                    except Exception as e:
                        print(f"⚠️  Failed to backup {file_path}: {e}")
                        continue
                
                conn.commit()
                print(f"✅ Successfully backed up {backup_count} session files")
                return True
                
        except Exception as e:
            print(f"❌ Session backup failed: {e}")
            return False
        finally:
            conn.close()
    
    def restore_session_files(self) -> bool:
        """Restore WhatsApp session files from database"""
        conn = self.get_db_connection()
        if not conn:
            return False
            
        try:
            with conn.cursor(cursor_factory=RealDictCursor) as cur:
                cur.execute("""
                    SELECT file_name, file_data, file_size, updated_at
                    FROM whatsapp_sessions 
                    WHERE project_name = %s
                    ORDER BY updated_at DESC
                """, (self.project_name,))
                
                rows = cur.fetchall()
                if not rows:
                    print("ℹ️  No session backup found in database")
                    return True
                    
                restore_count = 0
                for row in rows:
                    try:
                        file_name = row['file_name']
                        file_data = bytes(row['file_data'])
                        file_size = row['file_size']
                        updated_at = row['updated_at']
                        
                        # Determine restore location based on file type
                        if file_name.endswith('.db'):
                            # Database files go to session directory
                            restore_path = self.session_dir / file_name
                        else:
                            # Other session files go to whatsapp-bridge directory
                            restore_path = self.whatsapp_bridge_dir / file_name
                        
                        # Restore file
                        restore_path.write_bytes(file_data)
                        restore_count += 1
                        
                        print(f"📂 Restored: {file_name} ({file_size} bytes) from {updated_at}")
                        
                    except Exception as e:
                        print(f"⚠️  Failed to restore {row['file_name']}: {e}")
                        continue
                
                print(f"✅ Successfully restored {restore_count} session files")
                return True
                
        except Exception as e:
            print(f"❌ Session restore failed: {e}")
            return False
        finally:
            conn.close()
    
    def monitor_and_backup(self, interval_seconds: int = 300):
        """Continuously monitor and backup session files"""
        print(f"🔄 Starting session monitoring (backup every {interval_seconds}s)")
        backup_count = 0

        while True:
            try:
                time.sleep(interval_seconds)
                backup_count += 1
                # Only log every 6th backup (every 30 minutes) to reduce log spam
                if backup_count % 6 == 0:
                    print(f"🔄 Session backup #{backup_count} completed")
                else:
                    # Silent backup for routine operations
                    pass
                self.backup_session_files()
            except KeyboardInterrupt:
                print("⏹️  Session monitoring stopped")
                break
            except Exception as e:
                print(f"⚠️  Session monitoring error: {e}")
                time.sleep(60)  # Wait before retry
    
    def cleanup_old_backups(self, keep_days: int = 7):
        """Remove old session backups to save space"""
        conn = self.get_db_connection()
        if not conn:
            return
            
        try:
            with conn.cursor() as cur:
                cur.execute("""
                    DELETE FROM whatsapp_sessions 
                    WHERE project_name = %s 
                    AND updated_at < CURRENT_TIMESTAMP - INTERVAL '%s days'
                """, (self.project_name, keep_days))
                
                deleted_count = cur.rowcount
                conn.commit()
                
                if deleted_count > 0:
                    print(f"🧹 Cleaned up {deleted_count} old session backups")
                    
        except Exception as e:
            print(f"⚠️  Cleanup failed: {e}")
        finally:
            conn.close()


def main():
    """Main entry point for session persistence operations"""
    if len(sys.argv) < 2:
        print("Usage: python session_persistence.py [init|backup|restore|monitor]")
        sys.exit(1)
    
    manager = SessionPersistenceManager()
    command = sys.argv[1].lower()
    
    if command == "init":
        success = manager.init_session_table()
        sys.exit(0 if success else 1)
        
    elif command == "backup":
        success = manager.backup_session_files()
        sys.exit(0 if success else 1)
        
    elif command == "restore":
        success = manager.restore_session_files()
        sys.exit(0 if success else 1)
        
    elif command == "monitor":
        manager.monitor_and_backup()
        
    elif command == "cleanup":
        manager.cleanup_old_backups()
        
    else:
        print(f"Unknown command: {command}")
        sys.exit(1)


if __name__ == "__main__":
    main()