import { useQuery } from '@tanstack/react-query';
import {
  mockFetchMetricAggregates,
  mockFetchIdleCandidates,
  mockFetchScaleToZeroCandidates,
  mockFetchCostEstimates,
  mockFetchCpuTrend,
  mockFetchRamTrend,
} from '@/modules/finops/finops-mocks';

const USE_MOCK = process.env.NEXT_PUBLIC_MOCK_MODE === 'true';

export function useMetricAggregates() {
  return useQuery({
    queryKey: ['finops', 'aggregates'],
    queryFn: () => (USE_MOCK ? mockFetchMetricAggregates() : Promise.resolve([])),
    staleTime: 60 * 1000,
  });
}

export function useIdleCandidates() {
  return useQuery({
    queryKey: ['finops', 'idle-candidates'],
    queryFn: () => (USE_MOCK ? mockFetchIdleCandidates() : Promise.resolve([])),
    staleTime: 60 * 1000,
  });
}

export function useScaleToZeroCandidates() {
  return useQuery({
    queryKey: ['finops', 'scale-to-zero'],
    queryFn: () => (USE_MOCK ? mockFetchScaleToZeroCandidates() : Promise.resolve([])),
    staleTime: 60 * 1000,
  });
}

export function useCostEstimates() {
  return useQuery({
    queryKey: ['finops', 'cost-estimates'],
    queryFn: () => (USE_MOCK ? mockFetchCostEstimates() : Promise.resolve([])),
    staleTime: 60 * 1000,
  });
}

export function useCpuTrend() {
  return useQuery({
    queryKey: ['finops', 'cpu-trend'],
    queryFn: () => (USE_MOCK ? mockFetchCpuTrend() : Promise.resolve([])),
    staleTime: 60 * 1000,
  });
}

export function useRamTrend() {
  return useQuery({
    queryKey: ['finops', 'ram-trend'],
    queryFn: () => (USE_MOCK ? mockFetchRamTrend() : Promise.resolve([])),
    staleTime: 60 * 1000,
  });
}
