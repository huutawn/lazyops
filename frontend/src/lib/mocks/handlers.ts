import { http, HttpResponse } from 'msw';
import { API_BASE } from '@/lib/mock-helpers';
import {
  projects,
  instances,
  meshNetworks,
  clusters,
  deploymentBindings,
  deployments,
  logEntries,
  traces,
  metrics,
  topologyNodes,
  topologyEdges,
} from '@/lib/mock-data';

export const mockHandlers = [
  http.get(`${API_BASE}/projects`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: projects } });
  }),

  http.get(`${API_BASE}/instances`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: instances } });
  }),

  http.get(`${API_BASE}/mesh-networks`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: meshNetworks } });
  }),

  http.get(`${API_BASE}/clusters`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: clusters } });
  }),

  http.get(`${API_BASE}/projects/:id/bindings`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: deploymentBindings } });
  }),

  http.get(`${API_BASE}/deployments`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: deployments } });
  }),

  http.get(`${API_BASE}/deployments/:id/logs`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: logEntries } });
  }),

  http.get(`${API_BASE}/traces`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: traces } });
  }),

  http.get(`${API_BASE}/metrics`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: { items: metrics } });
  }),

  http.get(`${API_BASE}/topology`, () => {
    return HttpResponse.json({
      success: true,
      message: 'ok',
      data: { nodes: topologyNodes, edges: topologyEdges },
    });
  }),
];
