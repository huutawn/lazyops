'use client';

import { useEffect } from 'react';

type ErrorBoundaryProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function GlobalErrorBoundary({ error, reset }: ErrorBoundaryProps) {
  useEffect(() => {
    console.error('Application error:', error);
  }, [error]);

  return (
    <html lang="en">
      <body>
        <div className="flex min-h-screen items-center justify-center p-8">
          <div className="w-full max-w-md rounded-xl border border-health-unhealthy/30 bg-lazyops-card p-8 text-center">
            <h2 className="mb-2 text-xl font-semibold text-health-unhealthy">Something went wrong</h2>
            <p className="mb-4 text-sm text-lazyops-muted">
              An unexpected error occurred. Please try refreshing the page.
            </p>
            <div className="flex justify-center gap-3">
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={reset}
              >
                Try again
              </button>
              <button
                type="button"
                className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-medium text-lazyops-muted transition-colors hover:text-lazyops-text"
                onClick={() => window.location.reload()}
              >
                Refresh page
              </button>
            </div>
            {process.env.NODE_ENV === 'development' && error.message && (
              <pre className="mt-4 overflow-x-auto rounded-lg bg-lazyops-bg-accent p-3 text-left text-xs text-lazyops-muted">
                {error.message}
              </pre>
            )}
          </div>
        </div>
      </body>
    </html>
  );
}
