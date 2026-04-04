'use client';

import { useSession } from '@/lib/auth/auth-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import { SkeletonPage } from '@/components/primitives/skeleton';

export default function DashboardPage() {
  const { data: session, isLoading } = useSession();

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Dashboard"
        subtitle="Project overview and quick actions"
      />

      <div className="flex flex-col gap-4">
        <SectionCard title="Session" description="Current authenticated user">
          {session ? (
            <div className="flex flex-col gap-3 text-sm">
              <div className="flex items-center gap-3">
                <span className="text-lazyops-muted">Name:</span>
                <span className="text-lazyops-text">{session.display_name}</span>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-lazyops-muted">Email:</span>
                <span className="text-lazyops-text">{session.email}</span>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-lazyops-muted">Role:</span>
                <StatusBadge label={session.role} variant="info" size="sm" />
              </div>
              <div className="flex items-center gap-3">
                <span className="text-lazyops-muted">Status:</span>
                <StatusBadge label={session.status} variant="success" size="sm" />
              </div>
            </div>
          ) : (
            <p className="text-sm text-lazyops-muted">No session data available.</p>
          )}
        </SectionCard>

        <SectionCard title="Placeholder" description="Full dashboard coming in Day 21.">
          <p className="text-sm text-lazyops-muted">
            This page demonstrates the authenticated session layer. All protected routes
            require a valid session cookie set through the BFF auth proxy.
          </p>
        </SectionCard>
      </div>
    </div>
  );
}
