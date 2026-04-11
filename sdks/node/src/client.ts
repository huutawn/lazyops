import * as http from 'http';
import * as https from 'https';
import { URL } from 'url';
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
export class LazyOpsClient {
  private config: Required<LazyOpsConfig>;

  constructor(config: LazyOpsConfig = {}) {
    this.config = {
      dnsServer: config.dnsServer ?? '127.0.0.1:5353',
      projectId: config.projectId ?? process.env.LAZYOPS_PROJECT_ID ?? '',
      baseDomain: config.baseDomain ?? 'lazyops.internal',
      timeout: config.timeout ?? 10000,
      debug: config.debug ?? false,
    };
  }

  /**
   * Resolve a service by name to its host and port
   */
  async resolve(serviceName: string): Promise<ServiceRecord> {
    const host = this.getServiceHost(serviceName);
    const port = this.getServicePort(serviceName);

    const url = `http://${host}:${port}`;

    const record: ServiceRecord = {
      name: serviceName,
      host,
      port,
      protocol: 'http',
      url,
      healthPath: '/health',
    };

    if (this.config.debug) {
      console.log(`[LazyOps] Resolved service "${serviceName}" → ${url}`);
    }

    return record;
  }

  /**
   * Get all known services from environment variables
   */
  discoverServices(): ServiceRecord[] {
    const services: ServiceRecord[] = [];
    const serviceNames = new Set<string>();

    // Collect service names from LAZYOPS_SERVICE_* and LAZYOPS_DEP_* env vars
    for (const [key, value] of Object.entries(process.env)) {
      if (key.startsWith('LAZYOPS_SERVICE_') && key.endsWith('_HOST')) {
        const alias = key
          .replace('LAZYOPS_SERVICE_', '')
          .replace('_HOST', '')
          .toLowerCase();
        serviceNames.add(alias);
      }
      if (key.startsWith('LAZYOPS_DEP_') && key.endsWith('_HOST')) {
        const alias = key
          .replace('LAZYOPS_DEP_', '')
          .replace('_HOST', '')
          .toLowerCase();
        serviceNames.add(alias);
      }
    }

    for (const name of serviceNames) {
      try {
        const record = this.resolveSync(name);
        if (record) {
          services.push(record);
        }
      } catch {
        // Skip services that can't be resolved
      }
    }

    return services;
  }

  /**
   * Check health of a service
   */
  async checkHealth(serviceName: string, healthPath?: string): Promise<ServiceHealth> {
    const service = await this.resolve(serviceName);
    const path = healthPath ?? service.healthPath ?? '/health';
    const url = new URL(path, service.url);

    const start = Date.now();

    return new Promise((resolve) => {
      const httpClient = url.protocol === 'https:' ? https : http;
      const req = httpClient.get(url.toString(), { timeout: this.config.timeout }, (res) => {
        const latencyMs = Date.now() - start;
        res.resume();
        res.on('end', () => {
          resolve({
            name: serviceName,
            healthy: res.statusCode !== undefined && res.statusCode < 400,
            latencyMs,
            checkedAt: new Date().toISOString(),
          });
        });
      });

      req.on('error', (error) => {
        resolve({
          name: serviceName,
          healthy: false,
          latencyMs: Date.now() - start,
          checkedAt: new Date().toISOString(),
          error: error.message,
        });
      });

      req.on('timeout', () => {
        req.destroy();
        resolve({
          name: serviceName,
          healthy: false,
          latencyMs: this.config.timeout,
          checkedAt: new Date().toISOString(),
          error: 'request timed out',
        });
      });
    });
  }

  /**
   * Make an HTTP request to a service
   */
  async fetch(serviceName: string, path: string, options?: http.RequestOptions): Promise<string> {
    const service = await this.resolve(serviceName);
    const url = new URL(path, service.url);

    return new Promise((resolve, reject) => {
      const httpClient = url.protocol === 'https:' ? https : http;
      const req = httpClient.get(url.toString(), {
        ...options,
        timeout: this.config.timeout,
      }, (res) => {
        let data = '';
        res.on('data', (chunk) => (data += chunk));
        res.on('end', () => resolve(data));
      });

      req.on('error', reject);
      req.on('timeout', () => {
        req.destroy();
        reject(new Error(`Request to ${serviceName}${path} timed out after ${this.config.timeout}ms`));
      });
    });
  }

  /**
   * Get the connection string for an internal service (database, redis, etc.)
   */
  getInternalServiceConnectionString(kind: string): string {
    const kindUpper = kind.toUpperCase();
    const url = process.env[`LAZYOPS_SERVICE_${kindUpper}_URL`];
    if (url) return url;

    const host = process.env[`LAZYOPS_SERVICE_${kindUpper}_HOST`];
    const port = process.env[`LAZYOPS_SERVICE_${kindUpper}_PORT`];
    if (host && port) return `${kind}://${host}:${port}`;

    throw new Error(`No connection info found for internal service: ${kind}`);
  }

  // Private helpers

  private getServiceHost(serviceName: string): string {
    const key = `LAZYOPS_SERVICE_${serviceName.toUpperCase()}_HOST`;
    const depKey = `LAZYOPS_DEP_${serviceName.toUpperCase()}_HOST`;
    return process.env[key] ?? process.env[depKey] ?? `${serviceName}.${this.config.projectId}.${this.config.baseDomain}`;
  }

  private getServicePort(serviceName: string): number {
    const key = `LAZYOPS_SERVICE_${serviceName.toUpperCase()}_PORT`;
    const depKey = `LAZYOPS_DEP_${serviceName.toUpperCase()}_PORT`;
    const portStr = process.env[key] ?? process.env[depKey];
    if (portStr) return parseInt(portStr, 10);

    // Default ports based on common conventions
    const defaultPorts: Record<string, number> = {
      frontend: 3000,
      backend: 8000,
      api: 8000,
      server: 8080,
      web: 80,
    };
    return defaultPorts[serviceName.toLowerCase()] ?? 8080;
  }

  private resolveSync(serviceName: string): ServiceRecord | null {
    try {
      const host = this.getServiceHost(serviceName);
      const port = this.getServicePort(serviceName);
      return {
        name: serviceName,
        host,
        port,
        protocol: 'http',
        url: `http://${host}:${port}`,
        healthPath: '/health',
      };
    } catch {
      return null;
    }
  }
}
