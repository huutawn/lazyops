/**
 * Represents a discovered service in the LazyOps mesh
 */
export interface ServiceRecord {
    /** Service name (e.g., "backend", "frontend") */
    name: string;
    /** Service host address */
    host: string;
    /** Service port */
    port: number;
    /** Service protocol ("http", "tcp", "grpc") */
    protocol: string;
    /** Full URL (for HTTP services) */
    url?: string;
    /** Health check endpoint */
    healthPath?: string;
    /** Whether the service is currently healthy */
    healthy?: boolean;
}
/**
 * Health status of a service
 */
export interface ServiceHealth {
    /** Service name */
    name: string;
    /** Whether the service is healthy */
    healthy: boolean;
    /** Response time in milliseconds */
    latencyMs?: number;
    /** Last checked timestamp */
    checkedAt?: string;
    /** Error message if unhealthy */
    error?: string;
}
/**
 * Internal service types for managed infrastructure
 */
export interface InternalService {
    /** Service kind (postgres, redis, mysql, rabbitmq) */
    kind: string;
    /** Connection string */
    connectionString: string;
    /** Host */
    host: string;
    /** Port */
    port: number;
    /** Database name (for SQL databases) */
    database?: string;
    /** Username */
    username?: string;
    /** Password (managed by LazyOps) */
    password?: string;
}
