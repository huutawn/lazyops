import * as http from 'http';
import { ServiceRecord, ServiceHealth } from './types';
export interface LazyOpsConfig {
    /** LazyOps DNS server address (default: 127.0.0.1:5353) */
    dnsServer?: string;
    /** Project ID for service discovery */
    projectId?: string;
    /** Base domain for service resolution (default: lazyops.internal) */
    baseDomain?: string;
    /** Request timeout in milliseconds (default: 10000) */
    timeout?: number;
    /** Enable debug logging */
    debug?: boolean;
}
/**
 * LazyOpsClient provides service discovery and communication APIs
 */
export declare class LazyOpsClient {
    private config;
    constructor(config?: LazyOpsConfig);
    /**
     * Resolve a service by name to its host and port
     */
    resolve(serviceName: string): Promise<ServiceRecord>;
    /**
     * Get all known services from environment variables
     */
    discoverServices(): ServiceRecord[];
    /**
     * Check health of a service
     */
    checkHealth(serviceName: string, healthPath?: string): Promise<ServiceHealth>;
    /**
     * Make an HTTP request to a service
     */
    fetch(serviceName: string, path: string, options?: http.RequestOptions): Promise<string>;
    /**
     * Get the connection string for an internal service (database, redis, etc.)
     */
    getInternalServiceConnectionString(kind: string): string;
    private getServiceHost;
    private getServicePort;
    private resolveSync;
}
