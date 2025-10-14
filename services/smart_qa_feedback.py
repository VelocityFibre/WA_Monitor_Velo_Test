#!/usr/bin/env python3
"""
Smart QA Feedback Communicator - Velo Test Only
Only sends feedback once per drop when first marked as incomplete.
Prevents spam by tracking which drops have already received feedback.
"""

import time
import logging
import sys
import os
from datetime import datetime
from typing import List, Dict

# Import the feedback tracker
from qa_feedback_tracker import FeedbackTracker

# Import existing QA feedback functionality
from qa_feedback_communicator import (
    get_incomplete_qa_reviews, create_feedback_message, 
    send_feedback_to_agent, get_missing_steps, PROJECTS
)

# Initialize logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('smart_qa_feedback.log'),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

def process_velo_test_feedback(dry_run: bool = False):
    """Process QA feedback for Velo Test group only, avoiding spam."""
    
    # Initialize the feedback tracker
    tracker = FeedbackTracker('velo_test_feedback_tracker.json')
    
    logger.info("🚀 Starting Smart QA Feedback for Velo Test...")
    logger.info(f"{'📋 DRY RUN MODE' if dry_run else '💾 LIVE MODE'}")
    
    # Get current tracker stats
    stats = tracker.get_stats()
    logger.info(f"📊 Feedback tracker: {stats['total_feedback_sent']} drops already processed")
    
    # Get incomplete QA reviews (last 24 hours)
    reviews = get_incomplete_qa_reviews(hours_back=24)
    
    if not reviews:
        logger.info("✅ No incomplete QA reviews found")
        return
    
    # Filter for Velo Test project only
    velo_reviews = [review for review in reviews if review.get('project') == 'Velo Test']
    
    if not velo_reviews:
        logger.info("✅ No incomplete QA reviews found for Velo Test")
        return
    
    logger.info(f"🔍 Found {len(velo_reviews)} incomplete drops in Velo Test")
    
    feedback_sent_count = 0
    skipped_count = 0
    
    # Process each review
    for review in velo_reviews:
        drop_number = review['drop_number']
        project = review['project']
        assigned_agent = review.get('assigned_agent', 'Not specified')
        
        # Check if feedback has already been sent for this drop
        if not tracker.should_send_feedback(drop_number, project):
            logger.info(f"⏭️  Skipping {drop_number} - feedback already sent")
            skipped_count += 1
            continue
        
        # Get missing steps
        missing_steps = get_missing_steps(review['steps'])
        
        if not missing_steps:
            logger.info(f"✅ {drop_number} - No missing steps, skipping")
            continue
        
        logger.info(f"🔍 {drop_number}: {len(missing_steps)} missing steps")
        
        # Create feedback message
        message = create_feedback_message(drop_number, missing_steps, project, assigned_agent)
        
        # Send feedback
        if dry_run:
            logger.info(f"🔍 DRY RUN: Would send feedback for {drop_number}")
            logger.info(f"Message preview: {message[:100]}...")
            tracker.mark_feedback_sent(drop_number, project)  # Mark even in dry run for testing
        else:
            success = send_feedback_to_agent(drop_number, project, message, dry_run=False)
            
            if success:
                # Mark as sent in our tracker
                tracker.mark_feedback_sent(drop_number, project)
                feedback_sent_count += 1
                logger.info(f"✅ Feedback sent and tracked for {drop_number}")
                
                # Rate limiting - wait between messages to avoid spam
                time.sleep(2)
            else:
                logger.error(f"❌ Failed to send feedback for {drop_number}")
    
    # Summary
    logger.info(f"📊 Summary:")
    logger.info(f"   New feedback messages sent: {feedback_sent_count}")
    logger.info(f"   Dropped already processed: {skipped_count}")
    logger.info(f"   Total drops with feedback: {tracker.get_stats()['total_feedback_sent']}")
    
    # Cleanup old entries (older than 30 days)
    tracker.cleanup_old_entries(days_old=30)
    
    logger.info("✅ Smart QA Feedback processing completed")

def monitor_qa_feedback(check_interval: int = 300, dry_run: bool = False):
    """Continuously monitor for new incomplete QA reviews."""
    
    logger.info(f"🔄 Starting QA Feedback Monitor for Velo Test")
    logger.info(f"⏰ Check interval: {check_interval} seconds")
    logger.info(f"{'📋 DRY RUN MODE' if dry_run else '💾 LIVE MODE'}")
    logger.info("=" * 70)
    
    while True:
        try:
            process_velo_test_feedback(dry_run=dry_run)
            logger.info(f"⏳ Next check in {check_interval} seconds...")
            time.sleep(check_interval)
        except KeyboardInterrupt:
            logger.info("🛑 Monitor stopped by user")
            break
        except Exception as e:
            logger.error(f"❌ Error in QA feedback monitor: {e}")
            logger.info(f"⏳ Retrying in {check_interval} seconds...")
            time.sleep(check_interval)

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="Smart QA Feedback Communicator for Velo Test")
    parser.add_argument("--dry-run", action="store_true", help="Run in dry-run mode (no actual messages sent)")
    parser.add_argument("--once", action="store_true", help="Run once instead of continuously")
    parser.add_argument("--interval", type=int, default=300, help="Check interval in seconds (default: 300)")
    
    args = parser.parse_args()
    
    if args.once:
        process_velo_test_feedback(dry_run=args.dry_run)
    else:
        monitor_qa_feedback(check_interval=args.interval, dry_run=args.dry_run)