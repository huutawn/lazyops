import { useQuery } from '@tanstack/react-query';
import {
  mockFetchMetricAggregates,
  mockFetchIdleCandidates,
  mockFetchScaleToZeroCandidates,
  mockFetchCostEstimates,
  mockFetchCpuTrend,
  mockFetchRamTrend,
} from '@/modules/finops/finops-mocks';

export function useMetricAggregates() {
  return useQuery({
    queryKey: ['finops', 'aggregates'],
    queryFn: () => mockFetchMetricAggregates(),
    staleTime: 60 * 1000,
  });
}

export function useIdleCandidates() {
  return useQuery({
    queryKey: ['finops', 'idle-candidates'],
    queryFn: () => mockFetchIdleCandidates(),
    staleTime: 60 * 1000,
  });
}

export function useScaleToZeroCandidates() {
  return useQuery({
    queryKey: ['finops', 'scale-to-zero'],
    queryFn: () => mockFetchScaleToZeroCandidates(),
    staleTime: 60 * 1000,
  });
}

export function useCostEstimates() {
  return useQuery({
    queryKey: ['finops', 'cost-estimates'],
    queryFn: () => mockFetchCostEstimates(),
    staleTime: 60 * 1000,
  });
}

export function useCpuTrend() {
  return useQuery({
    queryKey: ['finops', 'cpu-trend'],
    queryFn: () => mockFetchCpuTrend(),
    staleTime: 60 * 1000,
  });
}

export function useRamTrend() {
  return useQuery({
    queryKey: ['finops', 'ram-trend'],
    queryFn: () => mockFetchRamTrend(),
    staleTime: 60 * 1000,
  });
}
