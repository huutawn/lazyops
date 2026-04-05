import { describe, expect, it } from 'vitest';
import { mockFetchTopology } from '@/modules/topology/topology-mocks';
import { normalizeTopologyResponse } from '@/modules/topology/topology-api';
import { mockFetchLogs, mockFetchIncidents, mockFetchMetrics } from '@/modules/observability/observability-mocks';
import { mockFetchMetricAggregates, mockFetchIdleCandidates, mockFetchCostEstimates } from '@/modules/finops/finops-mocks';
import { adaptTopology } from '@/modules/topology/topology-adapter';

describe('topology mocks', () => {
  it('returns topology data with nodes and edges', async () => {
    const result = await mockFetchTopology('proj_01');
    expect(result.nodes.length).toBeGreaterThan(0);
    expect(result.edges.length).toBeGreaterThan(0);
  });

  it('returns standalone topology for unknown project', async () => {
    const raw = await mockFetchTopology('unknown');
    const result = await adaptTopology(raw, 'mock');
    expect(result.runtime_mode).toBe('standalone');
  });
});

describe('topology adapter', () => {
  it('normalizes raw topology to display format', async () => {
    const raw = await mockFetchTopology('proj_01');
    const result = await adaptTopology(raw, 'mock');
    expect(result.nodes.length).toBeGreaterThan(0);
    expect(result.edges.length).toBeGreaterThan(0);
    expect(result.runtime_mode).toBeDefined();
    expect(result.displayConfig).toBeDefined();
    expect(result.source).toBe('mock');
  });

  it('applies runtime-mode-specific display rules', async () => {
    const raw = await mockFetchTopology('proj_01');
    const result = await adaptTopology(raw, 'mock');
    const config = result.displayConfig;
    if (result.runtime_mode === 'distributed-mesh') {
      expect(config.layout).toBe('force');
      expect(config.showLatency).toBe(true);
    }
  });
});

describe('topology API normalizer', () => {
  it('normalizes backend TopologyGraphResponse', () => {
    const raw = {
      project_id: 'proj_01',
      nodes: [
        { id: 'tn_01', project_id: 'proj_01', node_kind: 'instance' as const, node_ref: 'inst_01', name: 'prod-web-01', status: 'healthy' as const, metadata: {}, updated_at: '2026-04-04T12:00:00Z' },
      ],
      edges: [
        { id: 'te_01', project_id: 'proj_01', source_id: 'tn_01', target_id: 'tn_02', edge_kind: 'dependency' as const, protocol: 'http' as const, metadata: {} },
      ],
    };
    const result = normalizeTopologyResponse(raw);
    expect(result.nodes[0].label).toBe('prod-web-01');
    expect(result.nodes[0].runtime_mode).toBe('standalone');
    expect(result.edges[0].source).toBe('tn_01');
  });
});

describe('observability mocks', () => {
  it('returns log entries', async () => {
    const logs = await mockFetchLogs();
    expect(logs.length).toBeGreaterThan(0);
    expect(logs[0]).toHaveProperty('level');
    expect(logs[0]).toHaveProperty('message');
  });

  it('returns incidents', async () => {
    const incidents = await mockFetchIncidents();
    expect(incidents.length).toBeGreaterThan(0);
    expect(incidents[0]).toHaveProperty('severity');
    expect(incidents[0]).toHaveProperty('status');
  });

  it('returns metrics', async () => {
    const metrics = await mockFetchMetrics();
    expect(metrics.length).toBeGreaterThan(0);
    expect(metrics[0]).toHaveProperty('cpu_p95');
    expect(metrics[0]).toHaveProperty('ram_p95');
  });
});

describe('finops mocks', () => {
  it('returns metric aggregates', async () => {
    const aggregates = await mockFetchMetricAggregates();
    expect(aggregates.length).toBeGreaterThan(0);
    expect(aggregates[0]).toHaveProperty('cpu_p95');
    expect(aggregates[0]).toHaveProperty('ram_avg');
  });

  it('returns idle candidates', async () => {
    const candidates = await mockFetchIdleCandidates();
    expect(candidates.length).toBeGreaterThan(0);
    expect(candidates[0]).toHaveProperty('recommendation');
  });

  it('returns cost estimates with caveats', async () => {
    const estimates = await mockFetchCostEstimates();
    expect(estimates.length).toBeGreaterThan(0);
    expect(estimates[0].caveats.length).toBeGreaterThan(0);
  });
});
