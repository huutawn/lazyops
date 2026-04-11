"""Main LazyOps client implementation."""

import os
import time
import urllib.request
import urllib.error
from typing import Optional
from dataclasses import dataclass, field

from lazyops.types import ServiceRecord, ServiceHealth


@dataclass
class LazyOpsConfig:
    """Configuration for the LazyOps client."""

    dns_server: str = "127.0.0.1:5353"
    """LazyOps DNS server address"""

    project_id: str = ""
    """Project ID for service discovery"""

    base_domain: str = "lazyops.internal"
    """Base domain for service resolution"""

    timeout: float = 10.0
    """Request timeout in seconds"""

    debug: bool = False
    """Enable debug logging"""


_DEFAULT_PORTS = {
    "frontend": 3000,
    "backend": 8000,
    "api": 8000,
    "server": 8080,
    "web": 80,
}


class LazyOpsClient:
    """LazyOpsClient provides service discovery and communication APIs."""

    def __init__(self, config: Optional[LazyOpsConfig] = None):
        self.config = config or LazyOpsConfig()
        if not self.config.project_id:
            self.config.project_id = os.environ.get("LAZYOPS_PROJECT_ID", "")

    def resolve(self, service_name: str) -> ServiceRecord:
        """Resolve a service by name to its host and port."""
        host = self._get_service_host(service_name)
        port = self._get_service_port(service_name)
        url = f"http://{host}:{port}"

        record = ServiceRecord(
            name=service_name,
            host=host,
            port=port,
            protocol="http",
            url=url,
            health_path="/health",
        )

        if self.config.debug:
            print(f"[LazyOps] Resolved service \"{service_name}\" → {url}")

        return record

    def discover_services(self) -> list[ServiceRecord]:
        """Get all known services from environment variables."""
        services = []
        service_names = set()

        # Collect service names from LAZYOPS_SERVICE_* and LAZYOPS_DEP_* env vars
        for key, value in os.environ.items():
            if key.startswith("LAZYOPS_SERVICE_") and key.endswith("_HOST"):
                alias = key.replace("LAZYOPS_SERVICE_", "").replace("_HOST", "").lower()
                service_names.add(alias)
            if key.startswith("LAZYOPS_DEP_") and key.endswith("_HOST"):
                alias = key.replace("LAZYOPS_DEP_", "").replace("_HOST", "").lower()
                service_names.add(alias)

        for name in service_names:
            try:
                record = self.resolve(name)
                services.append(record)
            except Exception:
                pass  # Skip services that can't be resolved

        return services

    def check_health(self, service_name: str, health_path: Optional[str] = None) -> ServiceHealth:
        """Check health of a service."""
        service = self.resolve(service_name)
        path = health_path or service.health_path
        url = f"{service.url}{path}"

        start = time.time()

        try:
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req, timeout=self.config.timeout) as response:
                latency_ms = (time.time() - start) * 1000
                return ServiceHealth(
                    name=service_name,
                    healthy=response.status < 400,
                    latency_ms=latency_ms,
                    checked_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                )
        except Exception as e:
            latency_ms = (time.time() - start) * 1000
            return ServiceHealth(
                name=service_name,
                healthy=False,
                latency_ms=latency_ms,
                checked_at=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                error=str(e),
            )

    def fetch(self, service_name: str, path: str, method: str = "GET") -> str:
        """Make an HTTP request to a service."""
        service = self.resolve(service_name)
        url = f"{service.url}{path}"

        try:
            req = urllib.request.Request(url, method=method)
            with urllib.request.urlopen(req, timeout=self.config.timeout) as response:
                return response.read().decode("utf-8")
        except urllib.error.URLError as e:
            raise ConnectionError(f"Failed to fetch {service_name}{path}: {e}") from e

    def get_internal_service_connection_string(self, kind: str) -> str:
        """Get the connection string for an internal service (database, redis, etc.)."""
        kind_upper = kind.upper()
        url = os.environ.get(f"LAZYOPS_SERVICE_{kind_upper}_URL")
        if url:
            return url

        host = os.environ.get(f"LAZYOPS_SERVICE_{kind_upper}_HOST")
        port = os.environ.get(f"LAZYOPS_SERVICE_{kind_upper}_PORT")
        if host and port:
            return f"{kind}://{host}:{port}"

        raise ValueError(f"No connection info found for internal service: {kind}")

    def _get_service_host(self, service_name: str) -> str:
        key = f"LAZYOPS_SERVICE_{service_name.upper()}_HOST"
        dep_key = f"LAZYOPS_DEP_{service_name.upper()}_HOST"
        return (
            os.environ.get(key)
            or os.environ.get(dep_key)
            or f"{service_name}.{self.config.project_id}.{self.config.base_domain}"
        )

    def _get_service_port(self, service_name: str) -> int:
        key = f"LAZYOPS_SERVICE_{service_name.upper()}_PORT"
        dep_key = f"LAZYOPS_DEP_{service_name.upper()}_PORT"
        port_str = os.environ.get(key) or os.environ.get(dep_key)
        if port_str:
            return int(port_str)
        return _DEFAULT_PORTS.get(service_name.lower(), 8080)
