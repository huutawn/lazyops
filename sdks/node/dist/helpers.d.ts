import { LazyOpsClient, LazyOpsConfig } from './client';
import { ServiceRecord } from './types';
/**
 * Create a configured LazyOps client
 */
export declare function createClient(config?: LazyOpsConfig): LazyOpsClient;
/**
 * Discover all services available in the current environment
 */
export declare function discoverServices(config?: LazyOpsConfig): ServiceRecord[];
/**
 * Get a database connection string for the specified internal service
 * Supports postgres, mysql, and other SQL databases
 */
export declare function connectDB(kind: 'postgres' | 'mysql' | string, config?: LazyOpsConfig): string;
/**
 * Get a Redis connection string
 */
export declare function connectRedis(config?: LazyOpsConfig): string;
