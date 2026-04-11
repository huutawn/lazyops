/**
 * lazyops-sdk - LazyOps Service Discovery and Communication SDK
 *
 * Provides clean APIs for discovering and communicating with services
 * in a LazyOps deployment, both in local development and production.
 */

export { LazyOpsClient, LazyOpsConfig } from './client';
export { ServiceRecord, ServiceHealth } from './types';
export { createClient, discoverServices, connectDB, connectRedis } from './helpers';
