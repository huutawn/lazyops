"""LazyOps SDK for Python

Provides clean APIs for discovering and communicating with services
in a LazyOps deployment, both in local development and production.

Usage:
    from lazyops import LazyOpsClient

    client = LazyOpsClient()
    backend = client.resolve("backend")
    response = client.fetch("backend", "/api/users")
"""

__version__ = "0.1.0"

from lazyops.client import LazyOpsClient
from lazyops.types import ServiceRecord, ServiceHealth

__all__ = ["LazyOpsClient", "ServiceRecord", "ServiceHealth"]
