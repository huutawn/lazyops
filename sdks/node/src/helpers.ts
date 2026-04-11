import { LazyOpsClient, LazyOpsConfig } from './client';
import { ServiceRecord } from './types';

/**
 * Create a configured LazyOps client
 */
export function createClient(config?: LazyOpsConfig): LazyOpsClient {
  return new LazyOpsClient(config);
}

/**
 * Discover all services available in the current environment
 */
export function discoverServices(config?: LazyOpsConfig): ServiceRecord[] {
  const client = new LazyOpsClient(config);
  return client.discoverServices();
}

/**
 * Get a database connection string for the specified internal service
 * Supports postgres, mysql, and other SQL databases
 */
export function connectDB(kind: 'postgres' | 'mysql' | string, config?: LazyOpsConfig): string {
  const client = new LazyOpsClient(config);
  return client.getInternalServiceConnectionString(kind);
}

/**
 * Get a Redis connection string
 */
export function connectRedis(config?: LazyOpsConfig): string {
  const client = new LazyOpsClient(config);
  return client.getInternalServiceConnectionString('redis');
}
