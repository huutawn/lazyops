import { API_BASE_URL } from './client';
import type { ApiResponse, ApiError } from '@/lib/types';

type RequestOptions = Omit<RequestInit, 'headers'> & {
  params?: Record<string, string>;
  headers?: Record<string, string>;
};

async function buildUrl(path: string, params?: Record<string, string>): Promise<string> {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  const base = `${API_BASE_URL}${normalizedPath}`;
  if (!params || Object.keys(params).length === 0) {
    return base;
  }

  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    searchParams.set(key, value);
  });
  return `${base}?${searchParams.toString()}`;
}

function isApiResponse<T>(body: unknown): body is ApiResponse<T> {
  return (
    typeof body === 'object' &&
    body !== null &&
    ('data' in body || 'error' in body)
  );
}

export async function apiFetch<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiResponse<T>> {
  const { params, headers: extraHeaders, ...rest } = options;
  const url = await buildUrl(path, params);

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...extraHeaders,
  };

  try {
    const response = await fetch(url, { ...rest, headers, credentials: 'include' });

    if (!response.ok) {
      const errorBody = await response.json().catch(() => null);
      const apiError: ApiError = {
        message: errorBody?.message ?? `Request failed with status ${response.status}`,
        code: errorBody?.error?.code ?? errorBody?.code,
        details: errorBody?.error?.details ?? errorBody?.details,
      };
      return { data: null, error: apiError };
    }

    const body = await response.json();

    if (isApiResponse<T>(body)) {
      return body;
    }

    return { data: body as T };
  } catch (error) {
    const apiError: ApiError = {
      message: error instanceof Error ? error.message : 'Network error',
    };
    return { data: null, error: apiError };
  }
}

export async function apiGet<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiResponse<T>> {
  return apiFetch<T>(path, { ...options, method: 'GET' });
}

export async function apiPost<T>(
  path: string,
  body: unknown,
  options: RequestOptions = {},
): Promise<ApiResponse<T>> {
  return apiFetch<T>(path, {
    ...options,
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function apiPut<T>(
  path: string,
  body: unknown,
  options: RequestOptions = {},
): Promise<ApiResponse<T>> {
  return apiFetch<T>(path, {
    ...options,
    method: 'PUT',
    body: JSON.stringify(body),
  });
}

export async function apiDelete<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiResponse<T>> {
  return apiFetch<T>(path, { ...options, method: 'DELETE' });
}
