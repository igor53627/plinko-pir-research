"""
HTTP request handlers for Plinko PIR server.

Implements secure API endpoints with proper error handling,
input validation, and privacy protection measures.
"""

import logging
from flask import request, jsonify
from typing import Dict, Any, Callable
from functools import wraps

from utils import (
    validate_index, validate_prf_key, compute_xor, Timer,
    setup_security_headers, log_security_event, sanitize_for_logging
)

logger = logging.getLogger(__name__)


def cors_middleware(func: Callable) -> Callable:
    """
    Add CORS headers to responses and handle OPTIONS requests.
    
    Args:
        func: Function to wrap
        
    Returns:
        Wrapped function with CORS support
    """
    @wraps(func)
    def wrapper(*args, **kwargs):
        # Handle OPTIONS requests
        if request.method == 'OPTIONS':
            response = jsonify({})
            _add_cors_headers(response)
            return response, 200
        
        # Call the actual function
        try:
            result = func(*args, **kwargs)
            
            # Handle different return types
            if isinstance(result, tuple):
                data, status_code = result
            else:
                data, status_code = result, 200
            
            # Create response
            if isinstance(data, dict):
                response = jsonify(data)
            else:
                response = data
            
            # Add CORS headers
            _add_cors_headers(response)
            
            return response, status_code
            
        except Exception as e:
            logger.error(f"Handler error in {func.__name__}: {e}")
            response = jsonify({"error": "Internal server error"})
            _add_cors_headers(response)
            return response, 500
    
    return wrapper


def _add_cors_headers(response) -> None:
    """Add CORS headers to response."""
    response.headers['Access-Control-Allow-Origin'] = '*'
    response.headers['Access-Control-Allow-Methods'] = 'GET, POST, OPTIONS'
    response.headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization'
    response.headers['Access-Control-Max-Age'] = '86400'
    
    # Add security headers
    setup_security_headers(response)


def validate_json_request(required_fields: list = None) -> Dict[str, Any]:
    """
    Validate that request contains valid JSON with required fields.
    
    Args:
        required_fields: List of required field names
        
    Returns:
        Parsed JSON data
        
    Raises:
        ValueError: If validation fails
    """
    if not request.is_json:
        raise ValueError("Request must contain JSON data")
    
    data = request.get_json()
    if data is None:
        raise ValueError("Invalid JSON data")
    
    if required_fields:
        missing_fields = [field for field in required_fields if field not in data]
        if missing_fields:
            raise ValueError(f"Missing required fields: {missing_fields}")
    
    return data


# Handler functions for each endpoint

def create_health_handler(server):
    """Create health check handler."""
    @cors_middleware
    def health_handler():
        """Health check endpoint."""
        try:
            health_status = server.health_check()
            return health_status, 200
        except Exception as e:
            logger.error(f"Health check failed: {e}")
            return {"error": "Health check failed"}, 500
    
    return health_handler


def create_plaintext_query_handler(server):
    """Create plaintext query handler."""
    @cors_middleware
    def plaintext_query_handler():
        """Handle plaintext queries for specific database indices."""
        try:
            # Validate request
            data = validate_json_request(['index'])
            index = data['index']
            
            # Validate index type and range
            if not isinstance(index, int):
                raise ValueError("Index must be an integer")
            
            validate_index(index, server.db_size)
            
            # Process query (without logging index for privacy)
            with Timer() as timer:
                value = server.database.get_entry(index)
            
            return {
                "value": value,
                "server_time_nanos": timer.elapsed_nanos()
            }, 200
            
        except ValueError as e:
            log_security_event("Invalid plaintext query", {
                "error": str(e),
                "client_ip": request.remote_addr
            })
            return {"error": str(e)}, 400
        except Exception as e:
            logger.error(f"Plaintext query error: {e}")
            return {"error": "Internal server error"}, 500
    
    return plaintext_query_handler


def create_full_set_query_handler(server):
    """Create full set query handler."""
    @cors_middleware
    def full_set_query_handler():
        """Handle full set queries using PRF keys."""
        try:
            # Validate request
            data = validate_json_request(['prf_key'])
            prf_key_hex = data['prf_key']
            
            # Validate PRF key format
            if not isinstance(prf_key_hex, str):
                raise ValueError("PRF key must be a hex string")
            
            try:
                prf_key_bytes = bytes.fromhex(prf_key_hex)
            except ValueError:
                raise ValueError("Invalid hex string for prf_key")
            
            validate_prf_key(prf_key_bytes)
            
            # Process query
            with Timer() as timer:
                result = server.full_set_query(prf_key_bytes)
            
            return result, 200
            
        except ValueError as e:
            log_security_event("Invalid full set query", {
                "error": str(e),
                "client_ip": request.remote_addr
            })
            return {"error": str(e)}, 400
        except Exception as e:
            logger.error(f"Full set query error: {e}")
            return {"error": "Internal server error"}, 500
    
    return full_set_query_handler


def create_set_parity_query_handler(server):
    """Create set parity query handler."""
    @cors_middleware
    def set_parity_query_handler():
        """Handle set parity queries for specific indices."""
        try:
            # Validate request
            data = validate_json_request(['indices'])
            indices = data['indices']
            
            # Validate indices format
            if not isinstance(indices, list):
                raise ValueError("Indices must be a list")
            
            if not indices:
                raise ValueError("Indices list cannot be empty")
            
            # Validate each index
            for idx in indices:
                if not isinstance(idx, int):
                    raise ValueError("All indices must be integers")
                validate_index(idx, server.db_size)
            
            # Process query
            with Timer() as timer:
                result = server.set_parity_query(indices)
            
            return result, 200
            
        except ValueError as e:
            log_security_event("Invalid set parity query", {
                "error": str(e),
                "client_ip": request.remote_addr
            })
            return {"error": str(e)}, 400
        except Exception as e:
            logger.error(f"Set parity query error: {e}")
            return {"error": "Internal server error"}, 500
    
    return set_parity_query_handler


def create_error_handler():
    """Create global error handler."""
    def error_handler(error):
        """Handle unexpected errors."""
        logger.error(f"Unhandled error: {error}")
        log_security_event("Unhandled server error", {
            "error_type": type(error).__name__,
            "error_message": str(error)
        })
        
        response = jsonify({"error": "Internal server error"})
        _add_cors_headers(response)
        return response, 500
    
    return error_handler


# Security middleware

def security_middleware(func):
    """Add security checks to handlers."""
    @wraps(func)
    def wrapper(*args, **kwargs):
        # Check for suspicious patterns
        client_ip = request.remote_addr
        user_agent = request.headers.get('User-Agent', '')
        
        # Log suspicious requests
        if len(request.data) > 1024 * 1024:  # > 1MB
            log_security_event("Large request", {
                "size": len(request.data),
                "client_ip": client_ip
            })
        
        # Rate limiting could be added here
        
        return func(*args, **kwargs)
    
    return wrapper


def setup_routes(app, server):
    """
    Setup all route handlers for the Flask app.
    
    Args:
        app: Flask application instance
        server: PlinkoPIRServer instance
    """
    # Create handlers
    health_handler = create_health_handler(server)
    plaintext_handler = create_plaintext_query_handler(server)
    full_set_handler = create_full_set_query_handler(server)
    set_parity_handler = create_set_parity_query_handler(server)
    error_handler_func = create_error_handler()
    
    # Register routes
    app.route('/health', methods=['GET', 'OPTIONS'])(health_handler)
    app.route('/query/plaintext', methods=['POST', 'OPTIONS'])(plaintext_handler)
    app.route('/query/fullset', methods=['POST', 'OPTIONS'])(full_set_handler)
    app.route('/query/setparity', methods=['POST', 'OPTIONS'])(set_parity_handler)
    
    # Register error handler
    app.errorhandler(Exception)(error_handler_func)
    
    logger.info("All routes registered successfully")