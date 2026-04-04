import type { ReactNode } from 'react';

import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { LoadingPage } from '@/components/primitives/loading';
import type { PageBoundaryProps } from '@/lib/types';

export function PageBoundary<T>({
  state,
  data,
  error,
  loadingLabel,
  emptyTitle = 'Nothing here yet',
  emptyDescription = 'There is no data to display.',
  errorTitle,
  errorMessage,
  renderSuccess,
  renderEmptyAction,
  renderErrorAction,
}: PageBoundaryProps<T>) {
  if (state === 'loading') {
    return <LoadingPage label={loadingLabel} />;
  }

  if (state === 'error') {
    return (
      <ErrorState
        title={errorTitle}
        message={errorMessage ?? error?.message}
        action={renderErrorAction}
      />
    );
  }

  if (state === 'empty') {
    return (
      <EmptyState
        title={emptyTitle}
        description={emptyDescription}
        action={renderEmptyAction}
      />
    );
  }

  if (state === 'success' && data !== undefined) {
    return renderSuccess(data) as ReactNode;
  }

  return null;
}
