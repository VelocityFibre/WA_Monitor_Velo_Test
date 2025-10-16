#!/usr/bin/env python3
"""
QA Feedback Communicator
Monitors qa_photo_reviews table for 'incomplete' entries and sends WhatsApp messages
to installation agents requesting missing photos for specific steps.
"""

import argparse
import psycopg2
import time
import logging
import sys
import os
from datetime import datetime, timedelta
from typing import List, Dict, Optional

# Add whatsapp module to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import whatsapp

# Initialize logger at module level
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('qa_feedback.log'),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

# Configuration
NEON_DB_URL = "postgresql://neondb_owner:npg_RIgDxzo4St6d@ep-damp-credit-a857vku0-pooler.eastus2.azure.neon.tech/neondb?sslmode=require&channel_binding=require"

# Project configurations with WhatsApp group JIDs
PROJECTS = {
    'Lawley': {
        'group_jid': '120363418298130331@g.us',
        'group_name': 'Lawley Activation 3'
    },
    'Velo Test': {
        'group_jid': '120363421664266245@g.us', 
        'group_name': 'Velo Test'
    },
    'Mohadin': {
        'group_jid': '120363421532174586@g.us',             # Feedback target = production group
        'group_name': 'Mohadin Activations 🥳',                # Production group name
        'communication_disabled': False,  # ✅ ENABLED for production group
        'monitor_group_jid': '120363420337039473@g.us',      # Monitor group (backup)
        'parallel_testing_mode': False     # Production mode - impacts production
    }
}

# SAFETY: Mohadin feedback now routes to PRODUCTION group
MOHADIN_COMMUNICATION_DISABLED = False

# Deduplication mechanism to prevent spam
sent_feedback_cache = set()  # Cache of (drop_number, project, message_hash) tuples
CACHE_CLEANUP_INTERVAL = 300  # Clean cache every 5 minutes
MAX_CACHE_SIZE = 1000

# QA Step descriptions for clear feedback
QA_STEPS = {
    'step_01_property_frontage': '1. Property Frontage Photo',
    'step_02_location_before_install': '2. Location Before Installation',
    'step_03_outside_cable_span': '3. Outside Cable Span',
    'step_04_home_entry_outside': '4. Home Entry Outside',
    'step_05_home_entry_inside': '5. Home Entry Inside',
    'step_06_fibre_entry_to_ont': '6. Fibre Entry to ONT',
    'step_07_patched_labelled_drop': '7. Patched & Labelled Drop',
    'step_08_work_area_completion': '8. Work Area Completion',
    'step_09_ont_barcode_scan': '9. ONT Barcode Scan',
    'step_10_ups_serial_number': '10. UPS Serial Number',
    'step_11_powermeter_reading': '11. Power Meter Reading',
    'step_12_powermeter_at_ont': '12. Power Meter at ONT',
    'step_13_active_broadband_light': '13. Active Broadband Light',
    'step_14_customer_signature': '14. Customer Signature'
}

def setup_logging():
    """Set up logging configuration (already done at module level)."""
    return logger

def get_incomplete_qa_reviews(hours_back: int = 24) -> List[Dict]:
    """Get QA reviews marked as incomplete from Google Sheets (PRIMARY) and database (fallback)."""
    
    # First try to get from Google Sheets (more accurate)
    try:
        from google.oauth2.service_account import Credentials
        from googleapiclient.discovery import build
        import os
        
        GSHEET_ID = os.getenv("GSHEET_ID", "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk")
        GOOGLE_APPLICATION_CREDENTIALS = os.getenv("GOOGLE_APPLICATION_CREDENTIALS")
        
        if GSHEET_ID and GOOGLE_APPLICATION_CREDENTIALS:
            logger.info("📊 Reading QA reviews from Google Sheets (primary source)")
            
            # Get Google Sheets service - handle both file path and JSON string
            try:
                credentials = Credentials.from_service_account_file(
                    GOOGLE_APPLICATION_CREDENTIALS,
                    scopes=["https://www.googleapis.com/auth/spreadsheets"]
                )
            except (FileNotFoundError, OSError):
                # If file doesn't exist, try as JSON string
                import json
                try:
                    credentials_info = json.loads(GOOGLE_APPLICATION_CREDENTIALS)
                    credentials = Credentials.from_service_account_info(
                        credentials_info,
                        scopes=["https://www.googleapis.com/auth/spreadsheets"]
                    )
                except json.JSONDecodeError as je:
                    raise Exception(f"Invalid JSON in credentials: {je}")

            service = build("sheets", "v4", credentials=credentials, cache_discovery=False)
            
            # Get sheet data from multiple sheets
            sheet_names = ["Velo Test", "Mohadin WA_Tool Monitor"]  # Use monitor tab for testing
            all_reviews = []
            
            for sheet_name in sheet_names:
                try:
                    result = service.spreadsheets().values().get(
                        spreadsheetId=GSHEET_ID,
                        range=f"{sheet_name}!A:X"
                    ).execute()
                    
                    values = result.get('values', [])
                    sheet_reviews = []
                    
                    # Process this sheet's data
                    sheet_reviews = process_sheet_data_for_qa(values, sheet_name)
                    all_reviews.extend(sheet_reviews)
                    
                except Exception as e:
                    logger.warning(f"Could not read {sheet_name} sheet: {e}")
            
            logger.info(f"📊 Found {len(all_reviews)} incomplete QA reviews from Google Sheets")
            return all_reviews
            
    except Exception as e:
        logger.warning(f"⚠️  Could not read from Google Sheets: {e}")
        logger.info("💾 Falling back to database-based QA reviews")
    
    # Fallback to database method
    try:
        conn = psycopg2.connect(NEON_DB_URL)
        cursor = conn.cursor()
        
        # Get incomplete reviews that haven't had feedback sent yet
        since_time = datetime.now() - timedelta(hours=hours_back)
        
        query = """
        SELECT 
            qa.drop_number,
            qa.project,
            qa.assigned_agent,
            qa.user_name,
            qa.step_01_property_frontage, qa.step_02_location_before_install,
            qa.step_03_outside_cable_span, qa.step_04_home_entry_outside,
            qa.step_05_home_entry_inside, qa.step_06_fibre_entry_to_ont,
            qa.step_07_patched_labelled_drop, qa.step_08_work_area_completion,
            qa.step_09_ont_barcode_scan, qa.step_10_ups_serial_number,
            qa.step_11_powermeter_reading, qa.step_12_powermeter_at_ont,
            qa.step_13_active_broadband_light, qa.step_14_customer_signature,
            qa.comment,
            qa.updated_at
        FROM qa_photo_reviews qa
        WHERE qa.incomplete = TRUE 
        AND qa.completed = FALSE
        AND (qa.feedback_sent IS NULL OR qa.feedback_sent < qa.updated_at)
        AND qa.updated_at >= %s
        ORDER BY qa.updated_at DESC
        """
        
        cursor.execute(query, (since_time,))
        results = cursor.fetchall()
        
        reviews = []
        for row in results:
            review = {
                'drop_number': row[0],
                'project': row[1],
                'assigned_agent': row[2],
                'user_name': row[3],
                'steps': {
                    'step_01_property_frontage': row[4],
                    'step_02_location_before_install': row[5],
                    'step_03_outside_cable_span': row[6],
                    'step_04_home_entry_outside': row[7],
                    'step_05_home_entry_inside': row[8],
                    'step_06_fibre_entry_to_ont': row[9],
                    'step_07_patched_labelled_drop': row[10],
                    'step_08_work_area_completion': row[11],
                    'step_09_ont_barcode_scan': row[12],
                    'step_10_ups_serial_number': row[13],
                    'step_11_powermeter_reading': row[14],
                    'step_12_powermeter_at_ont': row[15],
                    'step_13_active_broadband_light': row[16],
                    'step_14_customer_signature': row[17]
                },
                'comment': row[18],
                'updated_at': row[19],
                'source': 'database'  # Mark as database-based
            }
            reviews.append(review)
        
        cursor.close()
        conn.close()
        
        logger.info(f"💾 Found {len(reviews)} incomplete QA reviews from database")
        return reviews
        
    except Exception as e:
        logger.error(f"❌ Error getting incomplete QA reviews from database: {e}")
        return []

def process_sheet_data_for_qa(values, sheet_name):
    """Process data from a specific sheet for QA reviews"""
    reviews = []
    
    # Parse each row looking for incomplete items
    for row_index, row in enumerate(values):
        if row_index <= 2:  # Skip header rows
            continue
            
        if len(row) < 22:  # Ensure row has enough columns
            continue
            
        drop_number = row[1].strip() if len(row) > 1 else ""
        if not drop_number or not drop_number.startswith('DR'):
            continue
        
        # Check if incomplete flag (Column V) is TRUE
        incomplete = row[21].lower() == 'true' if len(row) > 21 else False
        completed = row[23].lower() == 'true' if len(row) > 23 else False
        
        if incomplete and not completed:
            # Parse QA steps from Google Sheets (ACTUAL status)
            qa_steps = {
                'step_01_property_frontage': row[2].lower() == 'true' if len(row) > 2 else False,
                'step_02_location_before_install': row[3].lower() == 'true' if len(row) > 3 else False,
                'step_03_outside_cable_span': row[4].lower() == 'true' if len(row) > 4 else False,
                'step_04_home_entry_outside': row[5].lower() == 'true' if len(row) > 5 else False,
                'step_05_home_entry_inside': row[6].lower() == 'true' if len(row) > 6 else False,
                'step_06_fibre_entry_to_ont': row[7].lower() == 'true' if len(row) > 7 else False,
                'step_07_patched_labelled_drop': row[8].lower() == 'true' if len(row) > 8 else False,
                'step_08_work_area_completion': row[9].lower() == 'true' if len(row) > 9 else False,
                'step_09_ont_barcode_scan': row[10].lower() == 'true' if len(row) > 10 else False,
                'step_10_ups_serial_number': row[11].lower() == 'true' if len(row) > 11 else False,
                'step_11_powermeter_reading': row[12].lower() == 'true' if len(row) > 12 else False,
                'step_12_powermeter_at_ont': row[13].lower() == 'true' if len(row) > 13 else False,
                'step_13_active_broadband_light': row[14].lower() == 'true' if len(row) > 14 else False,
                'step_14_customer_signature': row[15].lower() == 'true' if len(row) > 15 else False,
            }
            
            reviews.append({
                'drop_number': drop_number,
                'project': sheet_name,
                'assigned_agent': row[18] if len(row) > 18 else 'Not specified',
                'user_name': row[18] if len(row) > 18 else 'Not specified',
                'steps': qa_steps,
                'comment': row[20] if len(row) > 20 else '',
                'updated_at': datetime.now(),
                'source': 'google_sheets'  # Mark as sheets-based
            })
    
    return reviews

def get_missing_steps(steps_dict: Dict[str, bool]) -> List[str]:
    """Identify which QA steps are missing (False)."""
    missing = []
    for step_key, is_complete in steps_dict.items():
        if not is_complete:
            step_description = QA_STEPS.get(step_key, step_key)
            missing.append(step_description)
    return missing

def create_feedback_message(drop_number: str, missing_steps: List[str], project: str, assigned_agent: str) -> str:
    """Create a WhatsApp feedback message for missing photos."""
    
    # Create a clear, professional message
    message = f"""🔍 QA REVIEW INCOMPLETE - {drop_number}

The following photos/steps need to be updated in 1MAP:

"""
    
    # Add numbered list of missing steps
    for i, step in enumerate(missing_steps, 1):
        message += f"• {step}\n"
    
    message += f"""
📋 Project: {project}
👤 Assigned Agent: {assigned_agent or 'Not specified'}

Please update the missing photos in 1MAP and resubmit.
Once updated, the QA team will re-review the installation.

Thank you! 📸✅"""

    return message

def send_feedback_to_agent(drop_number: str, project: str, message: str, dry_run: bool = False) -> bool:
    """Send feedback message to the group chat where the drop number was originally posted."""

    try:
        # Deduplication check
        import hashlib
        message_hash = hashlib.md5(message.encode()).hexdigest()[:8]  # Short hash for cache
        cache_key = (drop_number, project, message_hash)

        global sent_feedback_cache
        if cache_key in sent_feedback_cache:
            logger.info(f"🔄 Skipping duplicate feedback for {drop_number} in {project}")
            return True

        project_config = PROJECTS.get(project)
        if not project_config:
            logger.error(f"❌ Unknown project: {project}")
            return False

        group_name = project_config['group_name']
        group_jid = project_config['group_jid']

        if dry_run:
            logger.info(f"🔍 DRY RUN: Would send to {group_name} group for {drop_number}:")
            logger.info(f"Message: {message}")
            return True

        # Add to cache before sending to prevent race conditions
        sent_feedback_cache.add(cache_key)

        # Clean cache if it gets too large
        if len(sent_feedback_cache) > MAX_CACHE_SIZE:
            # Keep only the most recent half
            sent_feedback_cache = set(list(sent_feedback_cache)[MAX_CACHE_SIZE//2:])

        # Send message to the group where the DR was originally posted
        success, response = whatsapp.send_message(group_jid, message)

        if success:
            logger.info(f"✅ Feedback sent to {group_name} group for {drop_number}")
            return True
        else:
            logger.error(f"❌ Failed to send feedback to {group_name} group: {response}")
            return False

    except Exception as e:
        logger.error(f"❌ Error sending message to agent for {drop_number}: {e}")
        return False

def send_feedback_to_group(project: str, message: str, dry_run: bool = False) -> bool:
    """Send feedback message to the appropriate WhatsApp group (LEGACY - use send_feedback_to_agent instead)."""

    project_config = PROJECTS.get(project)
    if not project_config:
        logger.error(f"❌ Unknown project: {project}")
        return False

    group_jid = project_config['group_jid']
    group_name = project_config['group_name']

    if dry_run:
        logger.info(f"🔍 DRY RUN: Would send to {group_name} ({group_jid}):")
        logger.info(f"Message: {message}")
        return True

    try:
        # Send message to WhatsApp group
        success, response = whatsapp.send_message(group_jid, message)

        if success:
            logger.info(f"✅ Feedback sent to {group_name}")
            return True
        else:
            logger.error(f"❌ Failed to send feedback to {group_name}: {response}")
            return False

    except Exception as e:
        logger.error(f"❌ Error sending message to {group_name}: {e}")
        return False

def mark_feedback_sent(drop_number: str) -> bool:
    """Mark that feedback has been sent for this drop number."""
    try:
        conn = psycopg2.connect(NEON_DB_URL)
        cursor = conn.cursor()
        
        cursor.execute("""
            UPDATE qa_photo_reviews 
            SET feedback_sent = CURRENT_TIMESTAMP
            WHERE drop_number = %s
        """, (drop_number,))
        
        conn.commit()
        cursor.close()
        conn.close()
        
        logger.info(f"📝 Marked feedback sent for {drop_number}")
        return True
        
    except Exception as e:
        logger.error(f"❌ Error marking feedback sent for {drop_number}: {e}")
        return False

def process_qa_feedback(dry_run: bool = False, hours_back: int = 24):
    """Main function to process QA feedback requests."""
    
    logger.info(f"🚀 Starting QA Feedback Communicator...")
    logger.info(f"{'📋 DRY RUN MODE' if dry_run else '💾 LIVE MODE'}")
    logger.info(f"⏰ Checking QA reviews from last {hours_back} hours")
    logger.info("=" * 60)
    
    # Get incomplete QA reviews
    incomplete_reviews = get_incomplete_qa_reviews(hours_back)
    
    if not incomplete_reviews:
        logger.info("✅ No incomplete QA reviews found requiring feedback")
        return
    
    logger.info(f"📋 Found {len(incomplete_reviews)} incomplete QA reviews")
    
    feedback_sent = 0
    
    for review in incomplete_reviews:
        drop_number = review['drop_number']
        project = review['project'] or 'Unknown'
        assigned_agent = review['assigned_agent'] or 'Not specified'
        
        # Get missing steps
        missing_steps = get_missing_steps(review['steps'])
        
        if not missing_steps:
            logger.info(f"ℹ️  {drop_number}: All steps complete, but marked incomplete - skipping")
            continue
        
        logger.info(f"🔍 {drop_number}: {len(missing_steps)} missing steps")
        
        # Create feedback message
        message = create_feedback_message(drop_number, missing_steps, project, assigned_agent)
        
        # Send feedback directly to the agent who submitted the drop number
        if send_feedback_to_agent(drop_number, project, message, dry_run):
            if not dry_run:
                mark_feedback_sent(drop_number)
            feedback_sent += 1
        
        # Small delay between messages
        if not dry_run:
            time.sleep(2)
    
    logger.info(f"📊 Summary: {feedback_sent} feedback messages sent")
    logger.info("✅ QA Feedback processing completed")

def main():
    global logger
    logger = setup_logging()
    
    parser = argparse.ArgumentParser(description='QA Feedback Communicator for Incomplete Installations')
    parser.add_argument('--dry-run', action='store_true', help='Preview mode - don\'t actually send messages')
    parser.add_argument('--hours', type=int, default=24, help='Hours back to check for incomplete reviews (default: 24)')
    
    args = parser.parse_args()
    
    try:
        process_qa_feedback(dry_run=args.dry_run, hours_back=args.hours)
    except KeyboardInterrupt:
        logger.info("⚠️  Process interrupted by user")
    except Exception as e:
        logger.error(f"❌ Unexpected error: {e}")

if __name__ == "__main__":
    main()