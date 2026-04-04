export type { ReactNode } from 'react';

export type ApiError = {
  message: string;
  code?: string;
  details?: Record<string, unknown>;
};

export type ApiResponse<T> = {
  data: T;
  error?: null;
} | {
  data?: null;
  error: ApiError;
};

export type PageState = 'loading' | 'success' | 'error' | 'empty';

export type PageBoundaryProps<T> = {
  state: PageState;
  data?: T;
  error?: ApiError;
  loadingLabel?: string;
  emptyTitle?: string;
  emptyDescription?: string;
  errorTitle?: string;
  errorMessage?: string;
  renderSuccess: (data: T) => React.ReactNode;
  renderEmptyAction?: React.ReactNode;
  renderErrorAction?: React.ReactNode;
};
