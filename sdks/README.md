# LazyOps SDKs

Official SDKs for service discovery and communication in LazyOps deployments.

## Node.js SDK (`lazyops-sdk`)

### Installation

```bash
npm install lazyops-sdk
```

### Usage

```typescript
import { LazyOpsClient, discoverServices, connectDB, connectRedis } from 'lazyops-sdk';

const client = new LazyOpsClient({
  projectId: 'my-project',
  debug: true,
});

// Resolve a service
const backend = await client.resolve('backend');
console.log(backend.url); // http://backend.my-project.lazyops.internal:8000

// Make HTTP requests
const users = await client.fetch('backend', '/api/users');

// Check health
const health = await client.checkHealth('backend');
console.log(health.healthy); // true

// Discover all services
const services = discoverServices();

// Get database connection string
const pgUrl = connectDB('postgres');
const redisUrl = connectRedis();
```

## Python SDK (`lazyops-sdk`)

### Installation

```bash
pip install lazyops-sdk
```

### Usage

```python
from lazyops import LazyOpsClient

client = LazyOpsClient()

# Resolve a service
backend = client.resolve("backend")
print(backend.url)  # http://backend.my-project.lazyops.internal:8000

# Make HTTP requests
response = client.fetch("backend", "/api/users")

# Check health
health = client.check_health("backend")
print(health.healthy)  # True

# Discover all services
services = client.discover_services()

# Get database connection string
pg_url = client.get_internal_service_connection_string("postgres")
redis_url = client.get_internal_service_connection_string("redis")
```

## How It Works

The SDK uses environment variables injected by LazyOps:

| Variable | Description |
|----------|-------------|
| `LAZYOPS_SERVICE_<NAME>_HOST` | Service hostname |
| `LAZYOPS_SERVICE_<NAME>_PORT` | Service port |
| `LAZYOPS_SERVICE_<NAME>_URL` | Full service URL |
| `LAZYOPS_DEP_<ALIAS>_HOST` | Dependency alias hostname |
| `LAZYOPS_DEP_<ALIAS>_PORT` | Dependency alias port |
| `LAZYOPS_DEP_<ALIAS>_URL` | Dependency alias URL |

When running locally with `transparent_proxy` enabled, these resolve to `localhost`. In production, they resolve to actual service endpoints via the embedded DNS server.

## Service Discovery Priority

1. `LAZYOPS_SERVICE_<NAME>_HOST/PORT/URL` — Explicit service env vars
2. `LAZYOPS_DEP_<ALIAS>_HOST/PORT/URL` — Dependency alias env vars
3. Convention-based DNS: `<service>.<project>.lazyops.internal`
4. Fallback default ports (frontend: 3000, backend: 8000, etc.)
