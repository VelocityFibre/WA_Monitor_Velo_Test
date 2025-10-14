#!/usr/bin/env python3
"""
Done Message Detector - Monitors WhatsApp groups for completion responses
=====================================================================

Monitors WhatsApp group chats for messages indicating work completion
and automatically marks drops as resubmitted in Google Sheets.
"""

import os
import time
import logging
import sys
import re
import sqlite3
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Set

# Google Sheets imports
try:
    from google.oauth2.service_account import Credentials
    from googleapiclient.discovery import build
    GOOGLE_AVAILABLE = True
except ImportError:
    GOOGLE_AVAILABLE = False
    print("❌ Google Sheets libraries not available. Install with:")
    print("uv add google-api-python-client google-auth")
    sys.exit(1)

# Import local modules
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import whatsapp

# Set up logger first
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('done_message_detector.log'),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

# Configuration
SCOPES = ["https://www.googleapis.com/auth/spreadsheets"]
GSHEET_ID = os.getenv("GSHEET_ID")
GOOGLE_APPLICATION_CREDENTIALS = os.getenv("GOOGLE_APPLICATION_CREDENTIALS")

# Project configurations with WhatsApp group JIDs
PROJECTS = {
    'Lawley': {
        'group_jid': '120363418298130331@g.us',
        'group_name': 'Lawley Activation 3',
        'sheet_name': 'Lawley WA_Tool Monitor'
    },
    'Velo Test': {
        'group_jid': '120363421664266245@g.us',
        'group_name': 'Velo Test',
        'sheet_name': 'Velo Test'
    },
    'Mohadin': {
        'group_jid': '120363421532174586@g.us',
        'group_name': 'Mohadin Activations 🥳',
        'sheet_name': 'Mohadin WA_Tool Monitor'
    }
}

# Messages database path
MESSAGES_DB_PATH = os.getenv('WHATSAPP_DB_PATH', os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'whatsapp-bridge', 'store', 'messages.db'))

# Done message patterns
DONE_PATTERNS = [
    r'\bdone\b',
    r'\bcompleted\b',
    r'\bfinished\b',
    r'\bsorted\b',
    r'\bsortered\b',
    r'\bregulateer\b',
    r'\bvoltooi\b',
    r'\bafgehandel\b',
    r'\bklaar\b',
    r'\bready\b',
    r'\balle plessing\b',
    r'\balle foto\'s\b',
    r'\ball photos\b',
    r'\ball pictures\b'
]

# State tracking
PROCESSED_DONE_RESPONSES = set()  # Cache of processed message IDs
CACHE_CLEANUP_INTERVAL = 3600  # Clean cache every hour
MAX_CACHE_SIZE = 1000

def setup_logging():
    """Set up logging configuration (already done at module level)"""
    return logger

def get_sheets_service():
    """Get Google Sheets service connection"""
    try:
        credentials = Credentials.from_service_account_file(
            GOOGLE_APPLICATION_CREDENTIALS, scopes=SCOPES
        )
        return build("sheets", "v4", credentials=credentials, cache_discovery=False)
    except Exception as e:
        logger.error(f"Failed to create Google Sheets service: {e}")
        return None

def is_done_message(content: str) -> bool:
    """Check if message content indicates work completion"""
    if not content or not content.strip():
        return False

    content_lower = content.lower().strip()

    # Check against all patterns
    for pattern in DONE_PATTERNS:
        if re.search(pattern, content_lower, re.IGNORECASE):
            logger.debug(f"Done pattern matched: '{pattern}' in '{content[:50]}...'")
            return True

    return False

def extract_drop_number_from_message(content: str) -> Optional[str]:
    """Extract DR number from message content"""
    # Look for patterns like DR0000001, DR:123, Drop: DR4567, etc.
    patterns = [
        r'DR(\d{1,7})',  # DR followed by 1-7 digits
        r'dr(\d{1,7})',  # dr followed by 1-7 digits (lowercase)
        r'DR:\s*(\d{1,7})',  # DR: followed by 1-7 digits
        r'dr:\s*(\d{1,7})',  # dr: followed by 1-7 digits (lowercase)
        r'Drop:\s*DR(\d{1,7})',  # Drop: DR followed by 1-7 digits
        r'drop:\s*dr(\d{1,7})',  # drop: dr followed by 1-7 digits (lowercase)
        r'Drop\s*(\d{1,7})',  # Drop followed by 1-7 digits
        r'drop\s*(\d{1,7})',  # drop followed by 1-7 digits (lowercase)
        r'drop\s*number\s*(\d{1,7})'  # drop number followed by 1-7 digits
    ]

    for pattern in patterns:
        match = re.search(pattern, content, re.IGNORECASE)
        if match:
            drop_num = match.group(1)
            # Pad to 7 digits if shorter, keep as is if already 7 digits
            if len(drop_num) < 7:
                drop_num = drop_num.zfill(7)
            drop_number = f"DR{drop_num}"
            logger.debug(f"Extracted drop number: {drop_number} from '{content[:50]}...'")
            return drop_number

    return None

def find_drop_number_in_context(content: str, recent_messages: List[str]) -> Optional[str]:
    """Find DR number in current message or recent context"""
    # First try to extract from current message
    drop_number = extract_drop_number_from_message(content)
    if drop_number:
        return drop_number

    # Then check recent messages for DR numbers
    for msg_content in recent_messages[:10]:  # Check last 10 messages
        drop_number = extract_drop_number_from_message(msg_content)
        if drop_number:
            logger.debug(f"Found drop number in context: {drop_number}")
            return drop_number

    return None

def mark_drop_resubmitted(drop_number: str, sheet_name: str) -> bool:
    """Mark a drop as resubmitted in Google Sheets"""
    try:
        service = get_sheets_service()
        if not service:
            logger.error(f"Could not get Google Sheets service")
            return False

        # Read the sheet to find the row
        result = service.spreadsheets().values().get(
            spreadsheetId=GSHEET_ID,
            range=f"{sheet_name}!A:X"
        ).execute()

        values = result.get('values', [])
        if not values:
            logger.warning(f"No data found in {sheet_name}")
            return False

        # Find the row with the drop number
        drop_row_index = None
        for i, row in enumerate(values):
            if len(row) > 1 and row[1] == drop_number:  # Column B is drop number
                drop_row_index = i
                break

        if drop_row_index is None:
            logger.warning(f"Drop number {drop_number} not found in {sheet_name}")
            return False

        # Update Column W (Resubmitted) to TRUE and Column V (Incomplete) to FALSE
        row_num = drop_row_index + 1  # Convert to 1-based indexing

        # Update resubmitted flag (Column W = column 23, 0-indexed)
        resubmitted_range = f"{sheet_name}!W{row_num}"
        resubmitted_data = [[True]]

        resubmitted_body = {
            'values': resubmitted_data
        }

        service.spreadsheets().values().update(
            spreadsheetId=GSHEET_ID,
            range=resubmitted_range,
            body=resubmitted_body,
            valueInputOption="USER_ENTERED"
        ).execute()

        # Also clear incomplete flag if it was set (Column V = column 22, 0-indexed)
        incomplete_range = f"{sheet_name}!V{row_num}"
        incomplete_data = [[False]]

        incomplete_body = {
            'values': incomplete_data
        }

        service.spreadsheets().values().update(
            spreadsheetId=GSHEET_ID,
            range=incomplete_range,
            body=incomplete_body,
            valueInputOption="USER_ENTERED"
        ).execute()

        logger.info(f"✅ Marked {drop_number} as resubmitted in {sheet_name} (Row {row_num})")
        return True

    except Exception as e:
        logger.error(f"❌ Error marking {drop_number} as resubmitted in {sheet_name}: {e}")
        return False

def get_recent_messages_from_chat(chat_jid: str, limit: int = 20) -> List[str]:
    """Get recent messages from a specific chat"""
    try:
        conn = sqlite3.connect(MESSAGES_DB_PATH)
        cursor = conn.cursor()

        cursor.execute("""
            SELECT content
            FROM messages
            WHERE chat_jid = ? AND content != ''
            ORDER BY timestamp DESC
            LIMIT ?
        """, (chat_jid, limit))

        results = cursor.fetchall()
        conn.close()

        # Return as list, reversed to get chronological order
        return [row[0] for row in reversed(results)]

    except Exception as e:
        logger.error(f"❌ Error getting recent messages from {chat_jid}: {e}")
        return []

def get_unprocessed_done_messages(chat_jid: str, hours_back: int = 24) -> List[Dict]:
    """Get unprocessed done messages from a specific chat"""
    try:
        conn = sqlite3.connect(MESSAGES_DB_PATH)
        cursor = conn.cursor()

        # Get messages from the last X hours
        since_time = datetime.now() - timedelta(hours=hours_back)

        cursor.execute("""
            SELECT id, content, timestamp, sender
            FROM messages
            WHERE chat_jid = ?
            AND content != ''
            AND timestamp >= ?
            ORDER BY timestamp ASC
        """, (chat_jid, since_time))

        results = cursor.fetchall()
        conn.close()

        processed_messages = []
        for msg_id, content, timestamp, sender in results:
            # Skip if already processed
            if msg_id in PROCESSED_DONE_RESPONSES:
                continue

            # Check if it's a done message
            if is_done_message(content):
                processed_messages.append({
                    'id': msg_id,
                    'content': content,
                    'timestamp': timestamp,
                    'sender': sender
                })

        return processed_messages

    except Exception as e:
        logger.error(f"❌ Error getting unprocessed done messages from {chat_jid}: {e}")
        return []

def process_done_responses(hours_back: int = 24):
    """Main function to process done responses from all groups"""

    logger.info("🚀 Starting Done Message Detector...")
    logger.info(f"⏰ Checking messages from last {hours_back} hours")
    logger.info(f"📊 Monitoring groups: {', '.join([proj['group_name'] for proj in PROJECTS.values()])}")
    logger.info("=" * 70)

    total_processed = 0
    total_marked_resubmitted = 0

    for project_name, config in PROJECTS.items():
        group_jid = config['group_jid']
        group_name = config['group_name']
        sheet_name = config['sheet_name']

        logger.info(f"🔍 Processing {group_name} group...")

        # Get unprocessed done messages
        done_messages = get_unprocessed_done_messages(group_jid, hours_back)

        if not done_messages:
            logger.info(f"   ✅ No new done messages in {group_name}")
            continue

        logger.info(f"   📋 Found {len(done_messages)} done messages in {group_name}")

        # Get recent messages for context
        recent_context = get_recent_messages_from_chat(group_jid, 50)

        for msg in done_messages:
            total_processed += 1
            msg_id = msg['id']
            content = msg['content']
            sender = msg['sender']

            logger.info(f"   🔍 Processing done message from {sender}: '{content[:50]}...'")

            # Find drop number in message or context
            drop_number = find_drop_number_in_context(content, recent_context)

            if drop_number:
                logger.info(f"   🎯 Found drop number: {drop_number}")

                # Mark as resubmitted in Google Sheets
                if mark_drop_resubmitted(drop_number, sheet_name):
                    total_marked_resubmitted += 1
                    logger.info(f"   ✅ Successfully marked {drop_number} as resubmitted")
                    
                    # Reset feedback tracking so drop can receive feedback again if marked incomplete
                    try:
                        from qa_feedback_tracker import FeedbackTracker
                        tracker = FeedbackTracker('velo_test_feedback_tracker.json')
                        tracker.reset_feedback_tracking(drop_number, project)
                        logger.info(f"   🔄 Reset feedback tracking for {drop_number}")
                    except Exception as e:
                        logger.warning(f"   ⚠️ Could not reset feedback tracking: {e}")

                    # Send confirmation message to group
                    confirmation_msg = f"✅ **Resubmission Recorded**\n\nDrop {drop_number} has been marked as resubmitted and will be re-reviewed by the QA team."

                    try:
                        success, response = whatsapp.send_message(group_jid, confirmation_msg)
                        if success:
                            logger.info(f"   📱 Sent confirmation message to {group_name}")
                        else:
                            logger.warning(f"   ⚠️ Could not send confirmation message: {response}")
                    except Exception as e:
                        logger.warning(f"   ⚠️ Error sending confirmation message: {e}")
                else:
                    logger.warning(f"   ❌ Failed to mark {drop_number} as resubmitted")
            else:
                logger.warning(f"   ❌ Could not determine drop number from message or context")

            # Mark as processed to avoid duplicates
            PROCESSED_DONE_RESPONSES.add(msg_id)

        # Clean cache if it gets too large
        if len(PROCESSED_DONE_RESPONSES) > MAX_CACHE_SIZE:
            logger.info("   🧹 Cleaning processed message cache")
            # Keep only the most recent half
            PROCESSED_DONE_RESPONSES.clear()

    logger.info(f"📊 Summary:")
    logger.info(f"   Total done messages processed: {total_processed}")
    logger.info(f"   Drops marked as resubmitted: {total_marked_resubmitted}")
    logger.info("✅ Done message processing completed")

def monitor_done_responses(check_interval: int = 300):
    """Main monitoring loop for done responses"""

    logger.info("🚀 Starting Done Message Detector Monitor...")
    logger.info(f"⏰ Check interval: {check_interval} seconds")
    logger.info("🎯 Monitoring for completion messages like 'done', 'completed', 'voltooi', etc.")
    logger.info("=" * 70)

    while True:
        try:
            process_done_responses(hours_back=6)  # Check last 6 hours
            time.sleep(check_interval)

        except KeyboardInterrupt:
            logger.info("⚠️  Received keyboard interrupt. Shutting down...")
            break
        except Exception as e:
            logger.error(f"❌ Error in monitoring loop: {e}")
            logger.info(f"⏰ Waiting {check_interval} seconds before retry...")
            time.sleep(check_interval)

    logger.info("🛑 Done Message Detector Monitor stopped")

def main():
    setup_logging()

    import argparse
    parser = argparse.ArgumentParser(description='Done Message Detector for Resubmissions')
    parser.add_argument('--once', action='store_true', help='Run once and exit (good for testing)')
    parser.add_argument('--interval', type=int, default=300, help='Check interval in seconds (default: 300)')
    parser.add_argument('--hours', type=int, default=6, help='Hours back to check (default: 6)')

    args = parser.parse_args()

    try:
        if args.once:
            process_done_responses(hours_back=args.hours)
        else:
            monitor_done_responses(args.interval)

    except KeyboardInterrupt:
        logger.info("⚠️  Process interrupted by user")
    except Exception as e:
        logger.error(f"❌ Unexpected error: {e}")

if __name__ == "__main__":
    main()