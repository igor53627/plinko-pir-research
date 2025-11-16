"""
Configuration management for Plinko PIR server.

Handles loading and validating server configuration
with security-focused defaults and validation.
"""

import os
import logging
from typing import Optional

logger = logging.getLogger(__name__)


class ConfigurationError(Exception):
    """Exception raised for configuration-related errors."""
    pass


class PlinkoConfig:
    """
    Configuration for Plinko PIR server.
    
    Provides secure configuration management with validation.
    """
    
    def __init__(self):
        """Initialize with default configuration."""
        # Default values (can be overridden by environment variables)
        self.port = int(os.getenv('PLINKO_PORT', '8080'))
        self.database_path = os.getenv('PLINKO_DATABASE_PATH', 'data/database.bin')
        self.database_wait_timeout = int(os.getenv('PLINKO_DATABASE_TIMEOUT', '60'))  # seconds
        self.log_level = os.getenv('PLINKO_LOG_LEVEL', 'INFO')
        
        # Validate configuration
        self._validate()
    
    def _validate(self) -> None:
        """Validate configuration parameters."""
        if self.port <= 0 or self.port > 65535:
            raise ConfigurationError(f"Invalid port: {self.port}")
        
        if self.database_wait_timeout < 0:
            raise ConfigurationError(f"Invalid database timeout: {self.database_wait_timeout}")
        
        if not self.database_path:
            raise ConfigurationError("Database path cannot be empty")
        
        valid_log_levels = ['DEBUG', 'INFO', 'WARNING', 'ERROR', 'CRITICAL']
        if self.log_level.upper() not in valid_log_levels:
            raise ConfigurationError(f"Invalid log level: {self.log_level}")
    
    def get_listen_address(self) -> str:
        """
        Get the listen address for the HTTP server.
        
        Returns:
            Address in format "host:port" (uses 0.0.0.0 for all interfaces)
        """
        return f"0.0.0.0:{self.port}"
    
    def setup_logging(self) -> None:
        """Setup logging configuration."""
        log_level = getattr(logging, self.log_level.upper())
        
        logging.basicConfig(
            level=log_level,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
            datefmt='%Y-%m-%d %H:%M:%S'
        )
        
        logger.info("Logging configured with level: %s", self.log_level)
    
    @classmethod
    def from_args(cls, args) -> 'PlinkoConfig':
        """
        Create configuration from command line arguments.
        
        Args:
            args: Parsed command line arguments
            
        Returns:
            Configuration instance
        """
        config = cls()
        
        # Override with command line arguments if provided
        if hasattr(args, 'port') and args.port is not None:
            config.port = args.port
        
        if hasattr(args, 'database_path') and args.database_path is not None:
            config.database_path = args.database_path
        
        if hasattr(args, 'database_timeout') and args.database_timeout is not None:
            config.database_wait_timeout = args.database_timeout
        
        # Re-validate after overrides
        config._validate()
        
        return config
    
    def __repr__(self):
        return (f"PlinkoConfig(port={self.port}, database_path='{self.database_path}', "
                f"database_timeout={self.database_wait_timeout}, log_level='{self.log_level}')")


def load_config() -> PlinkoConfig:
    """
    Load configuration from environment variables and defaults.
    
    Returns:
        Loaded and validated configuration
        
    Raises:
        ConfigurationError: If configuration is invalid
    """
    return PlinkoConfig()