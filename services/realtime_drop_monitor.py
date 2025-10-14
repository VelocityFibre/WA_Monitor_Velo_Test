#!/usr/bin/env python3
"""
Real-time WhatsApp Drop Number Monitor
Continuously monitors the Lawley Activation 3 group for new drop numbers
and automatically syncs them to Neon database.

Usage: uv run python realtime_drop_monitor.py [--interval SECONDS] [--dry-run]
"""

import argparse
import re
import time
import sqlite3
import psycopg2
from datetime import datetime, timedelta
from typing import Set, List, Dict, Optional
import logging
import os
import signal
import sys
import json
from resubmission_handler import handle_drop_resubmission

# Google Sheets imports
try:
    from google.oauth2.service_account import Credentials
    from googleapiclient.discovery import build
    GOOGLE_AVAILABLE = True
except ImportError:
    GOOGLE_AVAILABLE = False
    logging.warning("Google Sheets libraries not available. Install with: pip install google-api-python-client google-auth")

# Configuration
# Project configurations
PROJECTS = {
    'Lawley': {
        'group_jid': '120363418298130331@g.us',
        'project_name': 'Lawley',
        'group_description': 'Lawley Activation 3 group'
    },
    'Velo Test': {
        'group_jid': '120363421664266245@g.us', 
        'project_name': 'Velo Test',
        'group_description': 'Velo Test group'
    },
    'Mohadin': {
        'group_jid': '120363421532174586@g.us',
        'project_name': 'Mohadin',
        'group_description': 'Mohadin Activations group'
    }
}

# Legacy variables for backward compatibility
LAWLEY_GROUP_JID = PROJECTS['Lawley']['group_jid']
VELO_TEST_GROUP_JID = PROJECTS['Velo Test']['group_jid']

MESSAGES_DB_PATH = os.getenv('WHATSAPP_DB_PATH', 'services/whatsapp-bridge/store/messages.db')
NEON_DB_URL = "postgresql://neondb_owner:npg_RIgDxzo4St6d@ep-damp-credit-a857vku0-pooler.eastus2.azure.neon.tech/neondb?sslmode=require&channel_binding=require"
DROP_PATTERN = r'DR\d+'

# Emergency kill switch - any user in monitored groups can trigger
KILL_COMMANDS = ["KILL", "!KILL", "kill all services", "emergency stop"]

# Google Sheets configuration
SCOPES = ["https://www.googleapis.com/auth/spreadsheets"]
GSHEET_ID = os.getenv("GSHEET_ID")
GOOGLE_APPLICATION_CREDENTIALS = os.getenv("GOOGLE_APPLICATION_CREDENTIALS")

# WhatsApp group to sheet mapping - Velo Test and Mohadin write to Google Sheets
SHEET_MAPPING = {
    '120363421664266245@g.us': 'Velo Test',                    # Velo Test group → Velo Test sheet
    '120363421532174586@g.us': 'Mohadin WA_Tool Monitor'      # Mohadin group → Monitor sheet (parallel testing)
    # Note: Lawley drops go to Neon only, not Google Sheets
    # Note: Mohadin writes to MONITOR tab for parallel testing (no impact on live sheet)
}

# Global variables for graceful shutdown
running = True
monitor_start_time = datetime.now()
STATE_FILE = 'monitor_state.json'

def setup_logging():
    """Set up logging configuration."""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(levelname)s - %(message)s',
        handlers=[
            logging.FileHandler('realtime_monitor.log'),
            logging.StreamHandler(sys.stdout)
        ]
    )
    return logging.getLogger(__name__)

def signal_handler(signum, frame):
    """Handle graceful shutdown."""
    global running
    logger.info(f"Received signal {signum}. Shutting down gracefully...")
    running = False

def emergency_stop_all_services():
    """Emergency stop all monitoring services immediately."""
    import subprocess
    logger.critical("🚨 EMERGENCY STOP TRIGGERED - Stopping all services")
    
    try:
        # Kill all monitoring processes
        subprocess.run(["pkill", "-f", "python.*monitor"], check=False)
        subprocess.run(["pkill", "-f", "whatsapp-bridge"], check=False)
        subprocess.run(["pkill", "-f", "google_sheets_qa_monitor"], check=False)
        logger.critical("🛑 All monitoring services stopped")
    except Exception as e:
        logger.error(f"Error during emergency stop: {e}")

def send_kill_confirmation(group_jid: str, sender: str):
    """Send confirmation message that kill command was received."""
    try:
        import whatsapp
        message = f"🛑 KILL COMMAND RECEIVED\n\nAll monitoring services stopped by: {sender}\nTime: {datetime.now().strftime('%H:%M:%S')}\n\nSystem is now OFFLINE."
        success, response = whatsapp.send_message(group_jid, message)
        if success:
            logger.info("✅ Kill confirmation sent")
        else:
            logger.warning(f"⚠️ Could not send kill confirmation: {response}")
    except Exception as e:
        logger.error(f"Error sending kill confirmation: {e}")

def check_for_kill_command(messages: List[Dict]) -> bool:
    """Check if any message contains a kill command."""
    for msg in messages:
        content = msg['content'].upper()
        sender = msg.get('sender', 'Unknown')
        chat_jid = msg.get('chat_jid', '')
        
        # Check for kill commands
        for kill_cmd in KILL_COMMANDS:
            if kill_cmd.upper() in content:
                logger.critical(f"🚨 KILL COMMAND DETECTED: '{kill_cmd}' from {sender}")
                
                # Send confirmation before stopping
                send_kill_confirmation(chat_jid, sender)
                
                # Stop all services
                emergency_stop_all_services()
                
                # Exit this process
                logger.critical("🛑 Realtime monitor exiting due to kill command")
                return True
    
    return False

def load_monitor_state() -> datetime:
    """Load the last check timestamp from state file."""
    try:
        if os.path.exists(STATE_FILE):
            with open(STATE_FILE, 'r') as f:
                state = json.load(f)
                last_check = datetime.fromisoformat(state['last_check_time'])
                logger.info(f"📂 Loaded last check time from state: {last_check}")
                return last_check
    except Exception as e:
        logger.warning(f"⚠️  Could not load state file: {e}")
    
    # Default to 1 hour ago to avoid processing too many old messages on first run
    default_time = datetime.now() - timedelta(hours=1)
    logger.info(f"🕐 Using default start time (1 hour ago): {default_time}")
    return default_time

def save_monitor_state(last_check_time: datetime, processed_ids: Set[str]):
    """Save the current monitor state to file."""
    try:
        # Keep only last 1000 IDs to prevent file from growing too large
        processed_list = list(processed_ids)
        if len(processed_list) > 1000:
            processed_list = processed_list[-1000:]
        
        state = {
            'last_check_time': last_check_time.isoformat(),
            'processed_message_ids': processed_list,
            'saved_at': datetime.now().isoformat()
        }
        with open(STATE_FILE, 'w') as f:
            json.dump(state, f, indent=2)
        logger.debug(f"💾 State saved: last_check={last_check_time}")
    except Exception as e:
        logger.error(f"❌ Could not save state: {e}")

def load_processed_message_ids() -> Set[str]:
    """Load previously processed message IDs from state file."""
    try:
        if os.path.exists(STATE_FILE):
            with open(STATE_FILE, 'r') as f:
                state = json.load(f)
                processed_ids = set(state.get('processed_message_ids', []))
                logger.info(f"📂 Loaded {len(processed_ids)} processed message IDs")
                return processed_ids
    except Exception as e:
        logger.warning(f"⚠️  Could not load processed IDs: {e}")
    
    return set()

def get_sheets_service():
    """Get Google Sheets service connection."""
    if not GOOGLE_AVAILABLE:
        return None
        
    try:
        credentials = Credentials.from_service_account_file(
            GOOGLE_APPLICATION_CREDENTIALS, scopes=SCOPES
        )
        return build("sheets", "v4", credentials=credentials, cache_discovery=False)
    except Exception as e:
        print(f"Failed to create Google Sheets service: {e}")
        try:
            logger.error(f"Failed to create Google Sheets service: {e}")
        except NameError:
            pass
        return None

def write_drop_to_google_sheets(drop_info: Dict, dry_run: bool = False) -> bool:
    """
    Write drop number to appropriate Google Sheets tab - called AFTER successful Neon write.
    
    Args:
        drop_info: Drop information dict (same as used for Neon)
        dry_run: If True, only log what would be done
        
    Returns:
        bool: Success status
    """
    if dry_run:
        print(f"🔍 DRY RUN: Would write {drop_info['drop_number']} to Google Sheets")
        try:
            logger.info(f"🔍 DRY RUN: Would write {drop_info['drop_number']} to Google Sheets")
        except NameError:
            pass
        return True
        
    if not GOOGLE_AVAILABLE or not GSHEET_ID or not GOOGLE_APPLICATION_CREDENTIALS:
        try:
            logger.debug("Google Sheets not configured, skipping sheets write")
        except NameError:
            pass
        return False
        
    try:
        service = get_sheets_service()
        if not service:
            return False
            
        # Determine which sheet tab to write to based on chat JID
        chat_jid = drop_info.get('chat_jid')
        sheet_name = SHEET_MAPPING.get(chat_jid, 'Unknown')
        if sheet_name == 'Unknown':
            print(f"Unknown chat JID {chat_jid}, skipping Google Sheets write")
            try:
                logger.warning(f"Unknown chat JID {chat_jid}, skipping Google Sheets write")
            except NameError:
                pass
            return False
            
        # Extract user name (same logic as Neon code)
        user_name = drop_info['contractor_name'].replace('WhatsApp-', '')[:20]
        
        # Prepare row data for the sheet
        today = datetime.now().strftime('%Y/%m/%d')
        comment = f"Auto-created from WhatsApp {sheet_name} group on {datetime.now().isoformat()}"
        
        # Row data matching your sheet structure (A-V columns)
        # Use boolean values for checkboxes (columns C-P, V)
        row_data = [
            today,                                    # A: Date
            drop_info['drop_number'],                # B: Drop Number  
            False, False, False, False, False, False, False,  # C-I: Steps 1-7 (checkboxes)
            False, False, False, False, False, False, False,  # J-P: Steps 8-14 (checkboxes)
            0,                                       # Q: Completed Photos
            14,                                      # R: Outstanding Photos
            user_name,                               # S: User
            "No",                                    # T: 1MAP Loaded
            comment,                                 # U: Comment
            False                                    # V: Incomplete (checkbox for resubmission)
        ]
        
        # Find the first empty row (start from row 3 since row 1-2 are headers)
        # Get current values to find first empty row
        current_values = service.spreadsheets().values().get(
            spreadsheetId=GSHEET_ID,
            range=f"{sheet_name}!A3:A1000"
        ).execute()
        
        existing_rows = current_values.get('values', [])
        first_empty_row = len(existing_rows) + 3  # +3 because we start from row 3
        
        # Write to the specific row (not append to bottom)
        service.spreadsheets().values().update(
            spreadsheetId=GSHEET_ID,
            range=f"{sheet_name}!A{first_empty_row}:V{first_empty_row}",
            valueInputOption="USER_ENTERED",
            body={"values": [row_data]}
        ).execute()
        
        # Now format columns C-P as checkboxes (steps 1-14)
        requests = []
        
        # Get sheet ID for the specific sheet tab
        sheet_metadata = service.spreadsheets().get(spreadsheetId=GSHEET_ID).execute()
        sheet_id = None
        for sheet in sheet_metadata['sheets']:
            if sheet['properties']['title'] == sheet_name:
                sheet_id = sheet['properties']['sheetId']
                break
        
        if sheet_id is not None:
            # Format columns C-P (indices 2-15) as checkboxes for QA steps
            requests.append({
                "repeatCell": {
                    "range": {
                        "sheetId": sheet_id,
                        "startRowIndex": first_empty_row - 1,  # Convert to 0-based
                        "endRowIndex": first_empty_row,
                        "startColumnIndex": 2,  # Column C (0-based)
                        "endColumnIndex": 16   # Column P (0-based, exclusive)
                    },
                    "cell": {
                        "dataValidation": {
                            "condition": {
                                "type": "BOOLEAN"
                            }
                        }
                    },
                    "fields": "dataValidation"
                }
            })
            
            # Format column V (index 21) as checkbox for "Incomplete" flag
            requests.append({
                "repeatCell": {
                    "range": {
                        "sheetId": sheet_id,
                        "startRowIndex": first_empty_row - 1,  # Convert to 0-based
                        "endRowIndex": first_empty_row,
                        "startColumnIndex": 21,  # Column V (0-based)
                        "endColumnIndex": 22    # Column V (0-based, exclusive)
                    },
                    "cell": {
                        "dataValidation": {
                            "condition": {
                                "type": "BOOLEAN"
                            }
                        }
                    },
                    "fields": "dataValidation"
                }
            })
            
            # Apply the formatting
            if requests:
                service.spreadsheets().batchUpdate(
                    spreadsheetId=GSHEET_ID,
                    body={"requests": requests}
                ).execute()
        
        print(f"📊 ✅ Also wrote {drop_info['drop_number']} to Google Sheets '{sheet_name}' tab")
        # Try to use logger if available, otherwise use print
        try:
            logger.info(f"📊 ✅ Also wrote {drop_info['drop_number']} to Google Sheets '{sheet_name}' tab")
        except NameError:
            pass
        return True
        
    except Exception as e:
        print(f"📊 ❌ Failed to write {drop_info['drop_number']} to Google Sheets: {e}")
        # Try to use logger if available, otherwise use print
        try:
            logger.error(f"📊 ❌ Failed to write {drop_info['drop_number']} to Google Sheets: {e}")
        except NameError:
            pass
        return False

def get_latest_messages_from_sqlite(since_timestamp: datetime, project_filter: str = None) -> Dict[str, List[Dict]]:
    """Get latest messages from WhatsApp SQLite database for all or specific projects."""
    try:
        conn = sqlite3.connect(MESSAGES_DB_PATH)
        cursor = conn.cursor()
        
        # Format timestamp for SQLite comparison
        # SQLite timestamps are stored as 'YYYY-MM-DD HH:MM:SS+TZ' format
        # We need to handle timezone-aware comparison properly
        if since_timestamp.tzinfo is None:
            # If since_timestamp has no timezone, assume local timezone
            from datetime import timezone, timedelta
            # Assume UTC+2 for consistency with stored timestamps
            since_timestamp = since_timestamp.replace(tzinfo=timezone(timedelta(hours=2)))
        
        timestamp_str = since_timestamp.strftime('%Y-%m-%d %H:%M:%S%z')
        # Convert to format that matches database (with colon in timezone)
        if len(timestamp_str) > 19 and timestamp_str[-2:].isdigit():
            timestamp_str = timestamp_str[:-2] + ':' + timestamp_str[-2:]
        
        logger.debug(f"📖 Querying SQLite for messages since: {timestamp_str}")
        
        project_messages = {}
        
        # Get projects to monitor
        projects_to_check = [project_filter] if project_filter else list(PROJECTS.keys())
        
        for project_name in projects_to_check:
            if project_name not in PROJECTS:
                continue
                
            project_config = PROJECTS[project_name]
            group_jid = project_config['group_jid']
            
            # Get messages from this project's group since the given timestamp
            cursor.execute("""
                SELECT id, content, sender, timestamp, is_from_me
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
                    'timestamp': datetime.fromisoformat(row[3]),
                    'is_from_me': bool(row[4]),
                    'project_name': project_name,
                    'chat_jid': group_jid
                })
            
            project_messages[project_name] = messages
            logger.debug(f"📖 Retrieved {len(messages)} messages for {project_name}")
        
        cursor.close()
        conn.close()
        
        total_messages = sum(len(msgs) for msgs in project_messages.values())
        logger.debug(f"📖 Retrieved {total_messages} messages total from SQLite")
        return project_messages
        
    except Exception as e:
        logger.error(f"Error reading from SQLite: {e}")
        return {}

def extract_drop_numbers_from_messages(messages: List[Dict]) -> List[Dict]:
    """Extract drop numbers from messages."""
    found_drops = []
    
    for msg in messages:
        content = msg['content']
        drops_in_message = re.findall(DROP_PATTERN, content, re.IGNORECASE)
        
        for drop in drops_in_message:
            drop_upper = drop.upper()
            
            # Create contractor name from sender
            sender = msg['sender']
            contractor_name = f'WhatsApp-{sender[:20]}...' if len(sender) > 20 else f'WhatsApp-{sender}'
            
            # Determine project-specific address description
            project_name = msg.get('project_name', 'Unknown')
            if project_name == 'Lawley':
                address = 'Extracted from WhatsApp Lawley Activation 3 group'
            elif project_name == 'Velo Test':
                address = 'Extracted from WhatsApp Velo Test group'
            else:
                address = f'Extracted from WhatsApp {project_name} group'
            
            found_drops.append({
                'drop_number': drop_upper,
                'message_id': msg['id'],
                'sender': sender,
                'contractor_name': contractor_name,
                'timestamp': msg['timestamp'],
                'message_content': content,
                'address': address,
                'project_name': project_name,
                'chat_jid': msg['chat_jid']
            })
    
    return found_drops

def get_existing_drop_numbers_from_neon() -> Set[str]:
    """Get existing DR drop numbers from Neon database."""
    try:
        conn = psycopg2.connect(NEON_DB_URL)
        cursor = conn.cursor()
        cursor.execute("SELECT drop_number FROM installations WHERE drop_number LIKE 'DR%'")
        existing = {row[0] for row in cursor.fetchall()}
        cursor.close()
        conn.close()
        return existing
    except Exception as e:
        logger.error(f"Error reading from Neon database: {e}")
        return set()

def create_qa_photo_review(drop_number: str, contractor_name: str, project_name: str = 'Lawley', dry_run: bool = False) -> bool:
    """Create a QA photo review entry for the drop number."""
    if dry_run:
        logger.info(f"🔍 DRY RUN: Would create QA photo review for {drop_number}")
        return True
    
    try:
        conn = psycopg2.connect(NEON_DB_URL)
        cursor = conn.cursor()
        
        # Check if QA review already exists for today
        cursor.execute("""
            SELECT id FROM qa_photo_reviews 
            WHERE drop_number = %s AND review_date = CURRENT_DATE
        """, (drop_number,))
        
        if cursor.fetchone():
            logger.debug(f"QA photo review already exists for {drop_number} today")
            cursor.close()
            conn.close()
            return True
        
        # Extract user name from contractor (remove WhatsApp- prefix)
        user_name = contractor_name.replace('WhatsApp-', '')[:20] if contractor_name.startswith('WhatsApp-') else contractor_name[:20]
        
        # Insert QA photo review with all steps defaulting to false
        insert_query = """
        INSERT INTO qa_photo_reviews (
            drop_number, 
            review_date,
            user_name,
            project,
            step_01_property_frontage, step_02_location_before_install,
            step_03_outside_cable_span, step_04_home_entry_outside,
            step_05_home_entry_inside, step_06_fibre_entry_to_ont,
            step_07_patched_labelled_drop, step_08_work_area_completion,
            step_09_ont_barcode_scan, step_10_ups_serial_number,
            step_11_powermeter_reading, step_12_powermeter_at_ont,
            step_13_active_broadband_light, step_14_customer_signature,
            outstanding_photos_loaded_to_1map,
            comment
        ) VALUES (
            %s, CURRENT_DATE, %s, %s,
            FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE,
            FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE,
            FALSE,
            %s
        )
        """
        
        comment = f"Auto-created from WhatsApp drop detection on {datetime.now().isoformat()}"
        
        cursor.execute(insert_query, (
            drop_number,
            user_name,
            project_name,
            comment
        ))
        
        conn.commit()
        cursor.close()
        conn.close()
        
        logger.info(f"📋 Created QA photo review for {drop_number} (user: {user_name})")
        return True
        
    except Exception as e:
        logger.error(f"❌ Error creating QA photo review for {drop_number}: {e}")
        return False

def insert_drop_numbers_to_neon(drop_data: List[Dict], dry_run: bool = False) -> int:
    """Insert new drop numbers into Neon database and create QA photo reviews."""
    if not drop_data:
        return 0
    
    if dry_run:
        logger.info(f"🔍 DRY RUN: Would insert {len(drop_data)} drop numbers:")
        for drop in drop_data:
            logger.info(f"   • {drop['drop_number']} from {drop['contractor_name']}")
            logger.info(f"   • Would create QA photo review for {drop['drop_number']}")
        return len(drop_data)
    
    try:
        conn = psycopg2.connect(NEON_DB_URL)
        cursor = conn.cursor()
        
        insert_query = """
        INSERT INTO installations (
            drop_number, 
            contractor_name, 
            address, 
            status, 
            agent_notes,
            project_name
        ) VALUES (%s, %s, %s, %s, %s, %s)
        """
        
        inserted_count = 0
        for drop_info in drop_data:
            try:
                cursor.execute(insert_query, (
                    drop_info['drop_number'],
                    drop_info['contractor_name'],
                    drop_info['address'],
                    'submitted',
                    f"Auto-imported from WhatsApp on {datetime.now().isoformat()} - "
                    f"Original timestamp: {drop_info['timestamp']} - "
                    f"Message: {drop_info['message_content'][:100]}...",
                    drop_info.get('project_name', 'Unknown')
                ))
                inserted_count += 1
                logger.info(f"✅ Inserted: {drop_info['drop_number']} from {drop_info['sender']}")
                
                # Create QA photo review for this drop
                create_qa_photo_review(
                    drop_info['drop_number'], 
                    drop_info['contractor_name'],
                    drop_info.get('project_name', 'Unknown'),
                    dry_run=False
                )
                
                # ✅ DUAL WRITE: Also write to Google Sheets after successful Neon write
                try:
                    write_drop_to_google_sheets(drop_info, dry_run=False)
                except Exception as e:
                    logger.error(f"Google Sheets dual-write failed for {drop_info['drop_number']}: {e}")
                    # Continue processing - don't fail if Sheets write fails
                
            except Exception as e:
                logger.error(f"❌ Error inserting {drop_info['drop_number']}: {e}")
        
        conn.commit()
        cursor.close()
        conn.close()
        
        if inserted_count > 0:
            logger.info(f"🎉 Successfully inserted {inserted_count} new drop numbers!")
        
        return inserted_count
        
    except Exception as e:
        logger.error(f"❌ Database error during insertion: {e}")
        return 0

def send_notification(drop_numbers: List[str], method: str = "log"):
    """Send notification about new drop numbers (placeholder for future implementation)."""
    message = f"🚨 NEW DROP NUMBERS DETECTED: {', '.join(drop_numbers)}"
    
    if method == "log":
        logger.info(f"📢 NOTIFICATION: {message}")
    
    # Future implementations could include:
    # - Email notifications
    # - Slack/Discord webhooks  
    # - WhatsApp messages to admin
    # - Desktop notifications

def monitor_and_sync(check_interval: int = 30, dry_run: bool = False):
    """Main monitoring loop."""
    global running, monitor_start_time
    
    logger.info("🚀 Starting Real-time Drop Number Monitor...")
    logger.info(f"{'📋 DRY RUN MODE' if dry_run else '💾 LIVE MODE'}")
    logger.info(f"⏰ Check interval: {check_interval} seconds")
    logger.info(f"👀 Monitoring groups:")
    for project_name, config in PROJECTS.items():
        logger.info(f"   • {project_name}: {config['group_jid']} ({config['group_description']})")
    logger.info("=" * 70)
    
    # Load persistent state
    last_check_time = load_monitor_state()
    processed_message_ids = load_processed_message_ids()
    
    while running:
        try:
            # Get new messages since last check for all projects
            project_messages = get_latest_messages_from_sqlite(last_check_time)
            
            # Flatten all messages for processing
            all_new_messages = []
            for project_name, messages in project_messages.items():
                all_new_messages.extend(messages)
            
            if all_new_messages:
                logger.info(f"📱 Found {len(all_new_messages)} new messages since {last_check_time.strftime('%Y-%m-%d %H:%M:%S')}")
                
                # 🚨 CHECK FOR KILL COMMAND FIRST (before any processing)
                if check_for_kill_command(all_new_messages):
                    # Kill command detected, function will handle stop and exit
                    sys.exit(0)
                
                # Log breakdown by project
                for project_name, messages in project_messages.items():
                    if messages:
                        logger.info(f"   • {project_name}: {len(messages)} messages")
                
                # Filter out messages we've already processed
                unprocessed_messages = [
                    msg for msg in all_new_messages 
                    if msg['id'] not in processed_message_ids
                ]
                
                if unprocessed_messages:
                    # Extract drop numbers from new messages
                    new_drops = extract_drop_numbers_from_messages(unprocessed_messages)
                    
                    if new_drops:
                        logger.info(f"🎯 Found {len(new_drops)} drop numbers in new messages:")
                        for drop in new_drops:
                            logger.info(f"   • {drop['drop_number']} - {drop['timestamp']} - {drop['sender']}")
                        
                        # Check which are actually new vs resubmissions
                        existing_in_neon = get_existing_drop_numbers_from_neon()
                        
                        truly_new_drops = []
                        resubmitted_drops = []
                        
                        for drop in new_drops:
                            if drop['drop_number'] in existing_in_neon:
                                resubmitted_drops.append(drop)
                            else:
                                truly_new_drops.append(drop)
                        
                        # Handle resubmissions first
                        if resubmitted_drops:
                            logger.info(f"🔄 {len(resubmitted_drops)} drop numbers are RESUBMISSIONS")
                            for drop in resubmitted_drops:
                                if not dry_run:
                                    handle_drop_resubmission(
                                        drop['drop_number'],
                                        drop['contractor_name'], 
                                        drop['project_name'],
                                        drop['message_content']
                                    )
                                else:
                                    logger.info(f"🔍 DRY RUN: Would handle resubmission {drop['drop_number']}")
                        
                        # Handle new drops
                        if truly_new_drops:
                            logger.info(f"🆕 {len(truly_new_drops)} drop numbers are new to database")
                            
                            # Send notification
                            new_drop_numbers = [drop['drop_number'] for drop in truly_new_drops]
                            send_notification(new_drop_numbers)
                            
                            # Insert into database
                            inserted = insert_drop_numbers_to_neon(truly_new_drops, dry_run)
                            
                            if inserted > 0:
                                logger.info(f"✅ Successfully synced {inserted} new drop numbers to database!")
                        
                        if not truly_new_drops and not resubmitted_drops:
                            logger.info("ℹ️  No actionable drop numbers found")
                    else:
                        logger.debug("No drop numbers found in new messages")
                    
                    # Mark messages as processed
                    for msg in unprocessed_messages:
                        processed_message_ids.add(msg['id'])
                else:
                    logger.debug("No unprocessed messages found")
            else:
                logger.debug(f"📶 No new messages since {last_check_time.strftime('%Y-%m-%d %H:%M:%S')}")
            
            # Update last check time to the most recent message timestamp or now
            if all_new_messages:
                last_check_time = max(msg['timestamp'] for msg in all_new_messages)
            else:
                last_check_time = datetime.now()
            
            # Save state after processing
            save_monitor_state(last_check_time, processed_message_ids)
            
            # Wait before next check
            if running:
                time.sleep(check_interval)
                
        except KeyboardInterrupt:
            logger.info("⚠️  Received keyboard interrupt. Shutting down...")
            break
        except Exception as e:
            logger.error(f"❌ Error in monitoring loop: {e}")
            logger.info(f"⏰ Waiting {check_interval} seconds before retry...")
            time.sleep(check_interval)
    
    logger.info("🛑 Monitor stopped.")
    
    # Print summary
    runtime = datetime.now() - monitor_start_time
    logger.info(f"📊 Monitor ran for {runtime}")
    logger.info("👋 Goodbye!")

def main():
    global logger
    
    # Set up logging
    logger = setup_logging()
    
    # Set up signal handlers for graceful shutdown
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    parser = argparse.ArgumentParser(description='Real-time WhatsApp drop number monitor')
    parser.add_argument('--interval', type=int, default=30, 
                       help='Check interval in seconds (default: 30)')
    parser.add_argument('--dry-run', action='store_true', 
                       help='Preview mode - don\'t actually insert into database')
    
    args = parser.parse_args()
    
    # Validate interval
    if args.interval < 5:
        logger.warning("⚠️  Minimum interval is 5 seconds. Setting to 5.")
        args.interval = 5
    
    # Check if WhatsApp database exists
    if not os.path.exists(MESSAGES_DB_PATH):
        logger.error(f"❌ WhatsApp database not found at {MESSAGES_DB_PATH}")
        logger.error("   Make sure the WhatsApp bridge is running and has created the database.")
        sys.exit(1)
    
    # Test connections
    logger.info("🔧 Testing database connections...")
    
    # Test SQLite connection
    try:
        conn = sqlite3.connect(MESSAGES_DB_PATH)
        conn.close()
        logger.info("✅ SQLite connection OK")
    except Exception as e:
        logger.error(f"❌ SQLite connection failed: {e}")
        sys.exit(1)
    
    # Test Neon connection
    try:
        conn = psycopg2.connect(NEON_DB_URL)
        conn.close()
        logger.info("✅ Neon database connection OK")
    except Exception as e:
        logger.error(f"❌ Neon database connection failed: {e}")
        sys.exit(1)
    
    # Check Google Sheets configuration (optional)
    if GOOGLE_AVAILABLE and GSHEET_ID and GOOGLE_APPLICATION_CREDENTIALS:
        try:
            service = get_sheets_service()
            if service:
                # Test by trying to get sheet metadata
                sheet_metadata = service.spreadsheets().get(spreadsheetId=GSHEET_ID).execute()
                logger.info(f"✅ Google Sheets connection OK - Sheet: '{sheet_metadata['properties']['title']}'")
                logger.info(f"📊 Google Sheets dual-write ENABLED for: {list(SHEET_MAPPING.values())}")
            else:
                logger.warning("⚠️  Google Sheets service creation failed - dual-write DISABLED")
        except Exception as e:
            logger.warning(f"⚠️  Google Sheets connection test failed: {e} - dual-write DISABLED")
    else:
        logger.info("📄 Google Sheets not configured - dual-write DISABLED")
        if not GOOGLE_AVAILABLE:
            logger.info("   Install: pip install google-api-python-client google-auth")
        if not GSHEET_ID:
            logger.info("   Set: export GSHEET_ID=your_sheet_id")
        if not GOOGLE_APPLICATION_CREDENTIALS:
            logger.info("   Set: export GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json")
    
    # Start monitoring
    monitor_and_sync(args.interval, args.dry_run)

if __name__ == "__main__":
    main()