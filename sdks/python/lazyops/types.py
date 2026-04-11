"""Type definitions for the LazyOps SDK."""

from dataclasses import dataclass, field
from typing import Optional


@dataclass
class ServiceRecord:
    """Represents a discovered service in the LazyOps mesh."""

    name: str
    """Service name (e.g., 'backend', 'frontend')"""

    host: str
    """Service host address"""

    port: int
    """Service port"""

    protocol: str = "http"
    """Service protocol ('http', 'tcp', 'grpc')"""

    url: Optional[str] = None
    """Full URL (for HTTP services)"""

    health_path: str = "/health"
    """Health check endpoint"""

    healthy: Optional[bool] = None
    """Whether the service is currently healthy"""

    def __post_init__(self):
        if self.url is None:
            scheme = "https" if self.protocol == "https" else "http"
            self.url = f"{scheme}://{self.host}:{self.port}"


@dataclass
class ServiceHealth:
    """Health status of a service."""

    name: str
    """Service name"""

    healthy: bool
    """Whether the service is healthy"""

    latency_ms: Optional[float] = None
    """Response time in milliseconds"""

    checked_at: Optional[str] = None
    """Last checked timestamp"""

    error: Optional[str] = None
    """Error message if unhealthy"""


@dataclass
class InternalService:
    """Internal service types for managed infrastructure."""

    kind: str
    """Service kind (postgres, redis, mysql, rabbitmq)"""

    connection_string: str
    """Connection string"""

    host: str
    """Host"""

    port: int
    """Port"""

    database: Optional[str] = None
    """Database name (for SQL databases)"""

    username: Optional[str] = None
    """Username"""

    password: Optional[str] = None
    """Password (managed by LazyOps)"""
