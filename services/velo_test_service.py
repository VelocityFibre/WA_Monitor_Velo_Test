#!/usr/bin/env python3
"""
Velo Test Isolated Service
Dedicated service for monitoring Velo Test WhatsApp group in production mode.
"""

import os
import sys
import logging
from pathlib import Path
from flask import Flask, jsonify
from threading import Thread
import time

# Add parent directory to path to access group_service_template
parent_dir = Path(__file__).parent.parent
sys.path.insert(0, str(parent_dir))

from group_service_template import GroupService

class VeloTestService:
    """
    Velo Test-specific service wrapper.
    Handles Velo Test group monitoring in production mode.
    """
    
    def __init__(self):
        self.service_id = "velo_test"
        self.port = 8082
        self.logger = self._setup_logging()
        
        # Set environment variables for this service
        os.environ['SERVICE_PORT'] = str(self.port)
        os.environ['CHECK_INTERVAL'] = '15'  # 15 second intervals
        
        # Initialize Flask app for health checks
        self.app = Flask(__name__)
        self.app.logger.disabled = True  # Disable Flask logging
        self._setup_routes()
        
        # Initialize the base service
        self.service = GroupService(self.service_id)
        self.service_started = False
        
    def _setup_logging(self) -> logging.Logger:
        """Setup Velo Test-specific logging."""
        logger = logging.getLogger('VeloTestService')
        logger.setLevel(logging.INFO)
        
        # Avoid duplicate handlers
        if not logger.handlers:
            handler = logging.StreamHandler()
            formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
            handler.setFormatter(formatter)
            logger.addHandler(handler)
            
        return logger
    
    def _setup_routes(self):
        """Setup Flask routes for health checks and status."""
        
        @self.app.route('/health', methods=['GET'])
        def health_check():
            """Health check endpoint."""
            return jsonify({
                'status': 'healthy',
                'service': 'velo_test',
                'port': self.port,
                'service_started': self.service_started
            })
            
        @self.app.route('/status', methods=['GET'])
        def status():
            """Detailed status endpoint."""
            return jsonify({
                'service_id': self.service_id,
                'port': self.port,
                'service_started': self.service_started,
                'mode': 'production',
                'safety': 'live_operations'
            })
    
    def _start_flask_app(self):
        """Start Flask app in a separate thread."""
        self.app.run(host='0.0.0.0', port=self.port, debug=False, use_reloader=False)
        
    def validate_configuration(self) -> bool:
        """Validate Velo Test-specific configuration."""
        config = self.service.config
        
        # Check required Velo Test configuration
        required_fields = [
            'whatsapp.group_jid',
            'whatsapp.feedback_target',
            'google_sheets.tab_name'
        ]
        
        for field in required_fields:
            keys = field.split('.')
            value = config
            for key in keys:
                value = value.get(key, {})
            
            if not value:
                self.logger.error(f"❌ Missing required configuration: {field}")
                return False
                
        # Validate production mode (not parallel testing)
        if config.get('whatsapp', {}).get('parallel_testing_mode', False):
            self.logger.warning("⚠️  Velo Test should be in production mode, not parallel testing")
            
        # Validate that feedback goes to same group (production mode)
        group_jid = config.get('whatsapp', {}).get('group_jid')
        feedback_target = config.get('whatsapp', {}).get('feedback_target')
        
        if group_jid != feedback_target:
            self.logger.warning(f"⚠️  Group JID ({group_jid}) differs from feedback target ({feedback_target})")
            
        self.logger.info("✅ Configuration validation passed")
        return True
        
    def print_startup_info(self):
        """Print startup information for Velo Test service."""
        config = self.service.config
        
        print("🧪 VELO TEST SERVICE STARTING")
        print("=" * 50)
        print(f"Service ID: {self.service_id}")
        print(f"Port: {self.port}")
        print(f"Mode: {'Production (Live)' if not config.get('whatsapp', {}).get('parallel_testing_mode') else 'Parallel Testing'}")
        print()
        
        print("📱 WHATSAPP CONFIGURATION:")
        print(f"  Group JID: {config.get('whatsapp', {}).get('group_jid')}")
        print(f"  Group Name: {config.get('whatsapp', {}).get('group_name')}")
        print(f"  Feedback Target: {config.get('whatsapp', {}).get('feedback_target')}")
        print()
        
        print("📊 GOOGLE SHEETS:")
        print(f"  Sheet ID: {config.get('google_sheets', {}).get('sheet_id')}")
        print(f"  Tab Name: {config.get('google_sheets', {}).get('tab_name')}")
        print()
        
        print("⚙️  SERVICE CONFIGURATION:")
        service_config = config.get('configuration', {})
        print(f"  Drop Detection: {'✅' if service_config.get('drop_detection_enabled') else '❌'}")
        print(f"  QA Feedback: {'✅' if service_config.get('qa_feedback_enabled') else '❌'}")
        print(f"  Google Sheets: {'✅' if service_config.get('google_sheets_write') else '❌'}")
        print(f"  Database Logging: {'✅' if service_config.get('database_logging') else '❌'}")
        print(f"  Communication Mode: {service_config.get('communication_mode', 'unknown')}")
        print()
        
        print("🎯 PRODUCTION STATUS:")
        print("  ✅ Monitors live Velo Test group")
        print("  ✅ Sends feedback to same group")
        print("  ✅ Writes to live Google Sheets tab")
        print("  ⚠️  Live production operations")
        print()
        
    def start(self):
        """Start the Velo Test service."""
        try:
            # Validate configuration before starting
            if not self.validate_configuration():
                self.logger.error("❌ Configuration validation failed. Cannot start service.")
                sys.exit(1)
                
            # Print startup information
            self.print_startup_info()
            
            # Start Flask app in background thread for health checks
            self.logger.info(f"🌐 Starting Flask health server on port {self.port}...")
            flask_thread = Thread(target=self._start_flask_app, daemon=True)
            flask_thread.start()
            
            # Give Flask a moment to start
            time.sleep(2)
            
            # Mark service as started
            self.service_started = True
            
            # Start the main monitoring service
            self.logger.info("🚀 Starting Velo Test monitoring service...")
            self.service.start()
            
        except KeyboardInterrupt:
            self.logger.info("🛑 Velo Test service interrupted by user")
        except Exception as e:
            self.logger.error(f"❌ Velo Test service failed: {e}")
            sys.exit(1)

def main():
    """Main entry point for Velo Test service."""
    print("🧪 Velo Test WhatsApp Group Monitoring Service")
    print("⚡ PRODUCTION MODE - Live monitoring with feedback to agents")
    print()
    
    velo_test_service = VeloTestService()
    velo_test_service.start()

if __name__ == "__main__":
    main()