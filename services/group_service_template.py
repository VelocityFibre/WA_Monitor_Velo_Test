#!/usr/bin/env python3
"""
Group Service Template for WA_Tool Microservices
Environment-configurable service that can monitor any WhatsApp group.
"""

import os
import sys
import json
import logging
import time
import sqlite3
import psycopg2
from datetime import datetime, timedelta, timezone
from typing import Dict, List, Optional, Any
import signal
import argparse
import re

# Add core modules to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'core'))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'whatsapp-mcp', 'whatsapp-mcp-server'))

class GroupService:
    """
    Generic service for monitoring WhatsApp groups.
    Configurable via environment variables or configuration file.
    """
    
    def __init__(self, service_id: str, config_path: str = None):
        self.service_id = service_id
        self.config_path = config_path or "/app/config/services.json"
        self.running = True
        self.logger = self._setup_logging()
        
        # Load configuration
        self.config = self._load_service_config()
        self.global_config = self._load_global_config()
        
        # Service state
        self.last_check_time = None
        self.processed_message_ids = set()
        self.check_interval = int(os.getenv('CHECK_INTERVAL', '15'))
        self.service_port = int(os.getenv('SERVICE_PORT', self.config.get('port', 8081)))
        
        # WhatsApp bridge configuration
        self.whatsapp_bridge_url = f"http://{self.global_config.get('whatsapp_bridge', {}).get('host', 'localhost')}:{self.global_config.get('whatsapp_bridge', {}).get('port', 8080)}"
        self.messages_db_path = "/app/store/messages.db"
        
        # Database connections
        self.neon_db_url = os.getenv('NEON_DB_URL', "postgresql://neondb_owner:npg_RIgDxzo4St6d@ep-damp-credit-a857vku0-pooler.eastus2.azure.neon.tech/neondb?sslmode=require&channel_binding=require")
        
        # Google Sheets configuration
        self.gsheet_id = os.getenv('GSHEET_ID', self.global_config.get('google_sheets', {}).get('default_sheet_id'))
        self.google_credentials_path = os.getenv('GOOGLE_APPLICATION_CREDENTIALS', self.global_config.get('google_sheets', {}).get('credentials_path'))
        
        # WhatsApp patterns
        self.drop_pattern = re.compile(r'DR\d+', re.IGNORECASE)
        
        # Setup signal handlers for graceful shutdown
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
        
    def _setup_logging(self) -> logging.Logger:
        """Setup service-specific logging."""
        logger = logging.getLogger(f'GroupService-{self.service_id}')
        logger.setLevel(logging.INFO)
        
        # Avoid duplicate handlers
        if not logger.handlers:
            # Console handler
            console_handler = logging.StreamHandler(sys.stdout)
            console_formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
            console_handler.setFormatter(console_formatter)
            logger.addHandler(console_handler)
            
            # File handler (with error handling)
            log_dir = "/app/logs"
            try:
                os.makedirs(log_dir, exist_ok=True)
                file_handler = logging.FileHandler(f"{log_dir}/{self.service_id}_service.log")
                file_formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
                file_handler.setFormatter(file_formatter)
                logger.addHandler(file_handler)
            except PermissionError:
                logger.warning(f"Cannot write to {log_dir}, using console logging only")
            
        return logger
        
    def _load_service_config(self) -> Dict:
        """Load service-specific configuration."""
        try:
            with open(self.config_path, 'r') as f:
                full_config = json.load(f)
                
            service_config = full_config.get('services', {}).get(self.service_id, {})
            if not service_config:
                raise ValueError(f"Service '{self.service_id}' not found in configuration")
                
            self.logger.info(f"✅ Loaded configuration for service '{self.service_id}'")
            return service_config
        except Exception as e:
            self.logger.error(f"❌ Failed to load service configuration: {e}")
            raise
            
    def _load_global_config(self) -> Dict:
        """Load global configuration."""
        try:
            with open(self.config_path, 'r') as f:
                full_config = json.load(f)
                
            return full_config.get('global_configuration', {})
        except Exception as e:
            self.logger.error(f"❌ Failed to load global configuration: {e}")
            return {}
            
    def _signal_handler(self, signum, frame):
        """Handle graceful shutdown signals."""
        self.logger.info(f"📡 Received signal {signum}. Shutting down gracefully...")
        self.running = False
        
    def get_whatsapp_group_jid(self) -> str:
        """Get WhatsApp group JID based on service configuration."""
        whatsapp_config = self.config.get('whatsapp', {})
        
        # For services with parallel testing mode (like Mohadin), monitor production but feedback to monitor
        if whatsapp_config.get('parallel_testing_mode', False):
            return whatsapp_config.get('production_group_jid')
        else:
            return whatsapp_config.get('group_jid')
            
    def get_feedback_target_jid(self) -> str:
        """Get JID where feedback messages should be sent."""
        whatsapp_config = self.config.get('whatsapp', {})
        return whatsapp_config.get('feedback_target')
        
    def get_google_sheets_tab(self) -> Optional[str]:
        """Get Google Sheets tab name for this service."""
        sheets_config = self.config.get('google_sheets', {})
        return sheets_config.get('tab_name')
        
    def is_drop_detection_enabled(self) -> bool:
        """Check if drop detection is enabled for this service."""
        return self.config.get('configuration', {}).get('drop_detection_enabled', True)
        
    def is_qa_feedback_enabled(self) -> bool:
        """Check if QA feedback is enabled for this service."""
        return self.config.get('configuration', {}).get('qa_feedback_enabled', True)
        
    def is_google_sheets_enabled(self) -> bool:
        """Check if Google Sheets integration is enabled."""
        return self.config.get('configuration', {}).get('google_sheets_write', False)
        
    def get_recent_messages(self, since_time: datetime) -> List[Dict]:
        """Get recent messages from WhatsApp bridge database."""
        try:
            conn = sqlite3.connect(self.messages_db_path)
            cursor = conn.cursor()
            
            group_jid = self.get_whatsapp_group_jid()
            if not group_jid:
                self.logger.error("❌ No WhatsApp group JID configured")
                return []
                
            # Add timezone info if missing
            if since_time.tzinfo is None:
                since_time = since_time.replace(tzinfo=timezone(timedelta(hours=2)))
                
            timestamp_str = since_time.strftime('%Y-%m-%d %H:%M:%S%z')
            # Convert to format that matches database (with colon in timezone)
            if len(timestamp_str) > 19 and timestamp_str[-2:].isdigit():
                timestamp_str = timestamp_str[:-2] + ':' + timestamp_str[-2:]
                
            cursor.execute("""
                SELECT id, content, sender, timestamp, is_from_me, chat_jid
                FROM messages 
                WHERE chat_jid = ? AND timestamp > ? AND content != ''
                ORDER BY timestamp ASC
            """, (group_jid, timestamp_str))
            
            messages = []
            for row in cursor.fetchall():
                messages.append({
                    'id': row[0],
                    'content': row[1],
                    'sender': row[2], 
                    'timestamp': row[3],
                    'is_from_me': bool(row[4]),
                    'chat_jid': row[5]
                })
                
            cursor.close()
            conn.close()
            
            self.logger.debug(f"📥 Retrieved {len(messages)} messages from {group_jid}")
            return messages
            
        except Exception as e:
            self.logger.error(f"❌ Error retrieving messages: {e}")
            return []
            
    def detect_drop_numbers(self, messages: List[Dict]) -> List[Dict]:
        """Detect drop numbers in messages."""
        drop_detections = []
        
        for message in messages:
            # Skip if already processed
            if message['id'] in self.processed_message_ids:
                continue
                
            # Look for drop numbers
            drop_matches = self.drop_pattern.findall(message['content'])
            
            if drop_matches:
                for drop_number in drop_matches:
                    drop_info = {
                        'drop_number': drop_number.upper(),
                        'message_id': message['id'],
                        'sender': message['sender'],
                        'content': message['content'],
                        'timestamp': message['timestamp'],
                        'chat_jid': message['chat_jid'],
                        'service_id': self.service_id
                    }
                    drop_detections.append(drop_info)
                    self.logger.info(f"🎯 Detected drop: {drop_number} from {message['sender']}")
                    
            # Mark message as processed
            self.processed_message_ids.add(message['id'])
            
        return drop_detections
        
    def write_to_google_sheets(self, drop_info: Dict) -> bool:
        """Write drop information to Google Sheets."""
        if not self.is_google_sheets_enabled():
            self.logger.debug("Google Sheets integration disabled for this service")
            return True
            
        try:
            from google.oauth2.service_account import Credentials
            from googleapiclient.discovery import build
            
            if not self.gsheet_id or not self.google_credentials_path:
                self.logger.warning("Google Sheets not configured")
                return False
                
            credentials = Credentials.from_service_account_file(
                self.google_credentials_path, 
                scopes=["https://www.googleapis.com/auth/spreadsheets"]
            )
            service = build("sheets", "v4", credentials=credentials, cache_discovery=False)
            
            tab_name = self.get_google_sheets_tab()
            if not tab_name:
                self.logger.warning("No Google Sheets tab configured for this service")
                return False
                
            # Prepare row data
            today = datetime.now().strftime('%Y/%m/%d')
            comment = f"Auto-created by {self.service_id} service on {datetime.now().isoformat()}"
            
            row_data = [
                today,                           # A: Date
                drop_info['drop_number'],       # B: Drop Number  
                False, False, False, False, False, False, False,  # C-I: Steps 1-7 (checkboxes)
                False, False, False, False, False, False, False,  # J-P: Steps 8-14 (checkboxes)
                0,                              # Q: Completed Photos
                14,                             # R: Outstanding Photos
                drop_info['sender'],            # S: Contractor Name
                'Processing',                   # T: Status
                '',                             # U: QA Notes
                '',                             # V: Comments
                False,                          # W: Resubmitted
                '',                             # X: Additional Notes
                False                           # Y: Incomplete (QA flag)
            ]
            
            # Append to sheet
            request_body = {
                'values': [row_data]
            }
            
            result = service.spreadsheets().values().append(
                spreadsheetId=self.gsheet_id,
                range=f"{tab_name}!A:Y",
                valueInputOption='USER_ENTERED',
                insertDataOption='INSERT_ROWS',
                body=request_body
            ).execute()
            
            self.logger.info(f"✅ Added {drop_info['drop_number']} to '{tab_name}' sheet")
            return True
            
        except Exception as e:
            self.logger.error(f"❌ Error writing to Google Sheets: {e}")
            return False
            
    def write_to_database(self, drop_info: Dict) -> bool:
        """Write drop information to Neon database."""
        try:
            conn = psycopg2.connect(self.neon_db_url)
            cursor = conn.cursor()
            
            # Check if drop already exists
            cursor.execute(
                "SELECT id FROM photo_submissions WHERE drop_number = %s AND project = %s",
                (drop_info['drop_number'], self.service_id)
            )
            
            if cursor.fetchone():
                self.logger.info(f"🔄 Drop {drop_info['drop_number']} already exists in database")
                cursor.close()
                conn.close()
                return True
                
            # Insert new drop
            cursor.execute("""
                INSERT INTO photo_submissions 
                (drop_number, project, contractor_name, user_name, message_id, whatsapp_group_id, 
                 created_at, status, notes)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
            """, (
                drop_info['drop_number'],
                self.service_id,
                drop_info['sender'],
                drop_info['sender'],
                drop_info['message_id'],
                drop_info['chat_jid'],
                datetime.now(),
                'pending',
                f"Auto-detected by {self.service_id} service"
            ))
            
            conn.commit()
            cursor.close()
            conn.close()
            
            self.logger.info(f"💾 Saved {drop_info['drop_number']} to database")
            return True
            
        except Exception as e:
            self.logger.error(f"❌ Error writing to database: {e}")
            return False
            
    def process_detected_drops(self, drops: List[Dict]) -> None:
        """Process detected drop numbers."""
        for drop_info in drops:
            success_count = 0
            
            # Write to database
            if self.write_to_database(drop_info):
                success_count += 1
                
            # Write to Google Sheets if enabled
            if self.is_google_sheets_enabled():
                if self.write_to_google_sheets(drop_info):
                    success_count += 1
                    
            if success_count > 0:
                self.logger.info(f"✅ Processed {drop_info['drop_number']} ({success_count} destinations)")
            else:
                self.logger.error(f"❌ Failed to process {drop_info['drop_number']}")
                
    def health_check(self) -> Dict:
        """Perform health check for this service."""
        status = {
            'service_id': self.service_id,
            'status': 'healthy',
            'port': self.service_port,
            'timestamp': datetime.now().isoformat(),
            'configuration': {
                'drop_detection': self.is_drop_detection_enabled(),
                'qa_feedback': self.is_qa_feedback_enabled(),
                'google_sheets': self.is_google_sheets_enabled(),
                'whatsapp_group': self.get_whatsapp_group_jid(),
                'feedback_target': self.get_feedback_target_jid(),
                'sheets_tab': self.get_google_sheets_tab()
            },
            'last_check': self.last_check_time.isoformat() if self.last_check_time else None,
            'processed_messages': len(self.processed_message_ids)
        }
        
        # Check WhatsApp bridge connectivity
        try:
            import requests
            response = requests.get(f"{self.whatsapp_bridge_url}/", timeout=5)
            # Bridge returns 404 but service is running if we get any response
            status['whatsapp_bridge'] = 'connected' if response.status_code in [200, 404] else 'disconnected'
        except Exception:
            status['whatsapp_bridge'] = 'disconnected'
            status['status'] = 'degraded'
            
        return status
        
    def run_monitoring_cycle(self) -> None:
        """Run a single monitoring cycle."""
        if not self.is_drop_detection_enabled():
            self.logger.debug("Drop detection disabled for this service")
            return
            
        # Set initial check time if not set
        if self.last_check_time is None:
            self.last_check_time = datetime.now() - timedelta(hours=1)
            self.logger.info(f"🕐 Initial check time set to: {self.last_check_time}")
            
        # Get recent messages
        messages = self.get_recent_messages(self.last_check_time)
        
        if messages:
            # Detect drop numbers
            drops = self.detect_drop_numbers(messages)
            
            if drops:
                self.logger.info(f"📋 Found {len(drops)} drop number(s) to process")
                self.process_detected_drops(drops)
            else:
                self.logger.debug("No new drop numbers detected")
        else:
            self.logger.debug("No new messages found")
            
        # Update last check time
        self.last_check_time = datetime.now()
        
    def start(self) -> None:
        """Start the group service."""
        self.logger.info(f"🚀 Starting {self.service_id} service on port {self.service_port}")
        self.logger.info(f"📱 Monitoring WhatsApp group: {self.get_whatsapp_group_jid()}")
        self.logger.info(f"💬 Feedback target: {self.get_feedback_target_jid()}")
        self.logger.info(f"📊 Google Sheets tab: {self.get_google_sheets_tab()}")
        self.logger.info(f"⏰ Check interval: {self.check_interval} seconds")
        
        try:
            while self.running:
                try:
                    self.run_monitoring_cycle()
                    time.sleep(self.check_interval)
                except KeyboardInterrupt:
                    self.logger.info("🛑 Keyboard interrupt received")
                    break
                except Exception as e:
                    self.logger.error(f"❌ Error in monitoring cycle: {e}")
                    time.sleep(self.check_interval)
                    
        except Exception as e:
            self.logger.error(f"❌ Fatal error: {e}")
        finally:
            self.logger.info(f"🛑 {self.service_id} service stopped")
            
def main():
    """Main entry point for group service."""
    parser = argparse.ArgumentParser(description='WhatsApp Group Monitoring Service')
    parser.add_argument('--service-id', required=True, help='Service identifier (mohadin, velo_test, lawley)')
    parser.add_argument('--config-path', help='Path to services.json configuration file')
    parser.add_argument('--check-interval', type=int, default=15, help='Check interval in seconds')
    parser.add_argument('--port', type=int, help='Service port number')
    
    args = parser.parse_args()
    
    # Set environment variables from args
    if args.port:
        os.environ['SERVICE_PORT'] = str(args.port)
    os.environ['CHECK_INTERVAL'] = str(args.check_interval)
    
    try:
        service = GroupService(args.service_id, args.config_path)
        service.start()
    except KeyboardInterrupt:
        print("\\n🛑 Service interrupted by user")
    except Exception as e:
        print(f"❌ Service failed to start: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()