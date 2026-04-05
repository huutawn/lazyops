import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="flex min-h-screen items-center justify-center p-8">
      <div className="w-full max-w-md rounded-xl border border-lazyops-border bg-lazyops-card p-8 text-center">
        <h2 className="mb-2 text-5xl font-bold text-primary">404</h2>
        <p className="mb-4 text-lg text-lazyops-text">Page not found</p>
        <p className="mb-6 text-sm text-lazyops-muted">
          The page you&apos;re looking for doesn&apos;t exist or has been moved.
        </p>
        <Link
          href="/dashboard"
          className="inline-block rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
        >
          Go to dashboard
        </Link>
      </div>
    </div>
  );
}
