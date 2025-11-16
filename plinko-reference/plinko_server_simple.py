#!/usr/bin/env python3
"""
Simplified Plinko PIR Server - Python Reference Implementation

A streamlined version that uses the handler modules for better organization.
"""

import argparse
import logging
from flask import Flask

from config import load_config, ConfigurationError
from database import PlinkoDatabase, DatabaseError
from plinko_core import PlinkoPIRServer  # We'll create this
from handlers import setup_routes

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)


def create_server(database_path: str, timeout: int = 60):
    """
    Create and initialize the Plinko PIR server.
    
    Args:
        database_path: Path to the database file
        timeout: Maximum time to wait for database
        
    Returns:
        Initialized server instance
    """
    logger.info("Creating Plinko PIR server...")
    
    # Create database and load it
    database = PlinkoDatabase(database_path)
    database.load(timeout)
    
    # Create core server
    from plinko_core import PlinkoPIRServer
    server = PlinkoPIRServer(database)
    
    logger.info(f"Server created successfully with {database.get_size()} entries")
    return server


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(description='Plinko PIR Server - Python Reference')
    parser.add_argument('--port', type=int, default=8080, help='Server port')
    parser.add_argument('--database-path', default='data/database.bin', help='Database file path')
    parser.add_argument('--database-timeout', type=int, default=60, help='Database wait timeout (seconds)')
    parser.add_argument('--log-level', choices=['DEBUG', 'INFO', 'WARNING', 'ERROR'], default='INFO')
    
    args = parser.parse_args()
    
    try:
        # Setup logging
        log_level = getattr(logging, args.log_level.upper())
        logging.basicConfig(level=log_level, format='%(asctime)s - %(levelname)s - %(message)s')
        
        # Log startup banner
        logger.info("=" * 50)
        logger.info("Plinko PIR Server - Python Reference")
        logger.info("=" * 50)
        logger.info("")
        logger.info(f"Configuration:")
        logger.info(f"  Port: {args.port}")
        logger.info(f"  Database: {args.database_path}")
        logger.info(f"  Timeout: {args.database_timeout}s")
        logger.info(f"  Log Level: {args.log_level}")
        
        # Create and initialize server
        server = create_server(args.database_path, args.database_timeout)
        
        # Setup routes
        setup_routes(app, server)
        
        logger.info("")
        logger.info(f"🚀 Starting server on http://0.0.0.0:{args.port}")
        logger.info("=" * 50)
        logger.info("")
        logger.info("Privacy Mode: ENABLED")
        logger.info("⚠️  Server will NEVER log queried addresses")
        logger.info("")
        
        # Run the server
        app.run(host='0.0.0.0', port=args.port, debug=False)
        
    except (ConfigurationError, DatabaseError) as e:
        logger.error(f"Startup error: {e}")
        exit(1)
    except KeyboardInterrupt:
        logger.info("Server stopped by user")
    except Exception as e:
        logger.error(f"Unexpected error: {e}")
        exit(1)


if __name__ == '__main__':
    main()