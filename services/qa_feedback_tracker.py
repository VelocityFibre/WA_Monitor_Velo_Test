#!/usr/bin/env python3
"""
QA Feedback Tracker
Prevents spam by ensuring feedback is only sent once per drop when first marked as incomplete.
"""

import json
import os
from datetime import datetime
from typing import Set, Dict

class FeedbackTracker:
    def __init__(self, tracker_file='qa_feedback_sent.json'):
        self.tracker_file = tracker_file
        self.feedback_sent = self._load_tracker()
        
    def _load_tracker(self) -> Dict[str, str]:
        """Load the feedback tracking data from file"""
        if os.path.exists(self.tracker_file):
            try:
                with open(self.tracker_file, 'r') as f:
                    data = json.load(f)
                    return data.get('feedback_sent', {})
            except Exception:
                pass
        return {}
    
    def _save_tracker(self):
        """Save the feedback tracking data to file"""
        data = {
            'feedback_sent': self.feedback_sent,
            'last_updated': datetime.now().isoformat()
        }
        try:
            with open(self.tracker_file, 'w') as f:
                json.dump(data, f, indent=2)
        except Exception as e:
            print(f"Warning: Could not save tracker file: {e}")
    
    def has_feedback_been_sent(self, drop_number: str, project: str) -> bool:
        """Check if feedback has already been sent for this drop"""
        key = f"{project}_{drop_number}"
        return key in self.feedback_sent
    
    def mark_feedback_sent(self, drop_number: str, project: str):
        """Mark that feedback has been sent for this drop"""
        key = f"{project}_{drop_number}"
        self.feedback_sent[key] = datetime.now().isoformat()
        self._save_tracker()
    
    def should_send_feedback(self, drop_number: str, project: str, force_resend: bool = False) -> bool:
        """
        Determine if feedback should be sent for this drop.
        Only send feedback once per drop unless force_resend is True.
        """
        if force_resend:
            return True
            
        return not self.has_feedback_been_sent(drop_number, project)
    
    def cleanup_old_entries(self, days_old: int = 30):
        """Remove tracking entries older than specified days"""
        cutoff_date = datetime.now().timestamp() - (days_old * 24 * 60 * 60)
        
        to_remove = []
        for key, timestamp_str in self.feedback_sent.items():
            try:
                timestamp = datetime.fromisoformat(timestamp_str).timestamp()
                if timestamp < cutoff_date:
                    to_remove.append(key)
            except Exception:
                # Remove invalid entries
                to_remove.append(key)
        
        for key in to_remove:
            del self.feedback_sent[key]
        
        if to_remove:
            self._save_tracker()
            print(f"Cleaned up {len(to_remove)} old feedback tracking entries")
    
    def reset_feedback_tracking(self, drop_number: str, project: str):
        """Reset feedback tracking for a drop when it's resubmitted.
        This allows the system to send feedback again if the drop is marked incomplete after resubmission.
        """
        key = f"{project}_{drop_number}"
        if key in self.feedback_sent:
            del self.feedback_sent[key]
            self._save_tracker()
            print(f"🔄 Reset feedback tracking for {drop_number} - can receive feedback again")
        else:
            print(f"ℹ️  No feedback tracking found for {drop_number} - already clear")
    
    def get_stats(self) -> Dict:
        """Get statistics about feedback tracking"""
        return {
            'total_feedback_sent': len(self.feedback_sent),
            'drops_with_feedback': list(self.feedback_sent.keys()),
            'tracker_file': self.tracker_file
        }

# Usage example and testing
if __name__ == "__main__":
    tracker = FeedbackTracker()
    
    # Test the functionality
    print("QA Feedback Tracker Test")
    print(f"Stats: {tracker.get_stats()}")
    
    # Test should_send_feedback
    test_drop = "DR1234567"
    test_project = "Velo Test"
    
    print(f"Should send feedback for {test_drop}? {tracker.should_send_feedback(test_drop, test_project)}")
    
    # Mark as sent
    tracker.mark_feedback_sent(test_drop, test_project)
    print(f"After marking as sent, should send again? {tracker.should_send_feedback(test_drop, test_project)}")
    
    print(f"Updated stats: {tracker.get_stats()}")