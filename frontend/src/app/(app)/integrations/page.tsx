import Link from 'next/link';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';

export default function IntegrationsPage() {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Integrations"
        subtitle="Connect external systems to automate source sync, build, and deployment."
      />

      <SectionCard title="Available integrations" description="GitHub App powers repository sync and webhook flow.">
        <div className="grid gap-4 sm:grid-cols-2">
          <Link
            href="/integrations/github"
            className="rounded-xl border border-lazyops-border bg-lazyops-card p-4 transition-colors hover:bg-lazyops-border/10"
          >
            <h3 className="mb-1 text-sm font-semibold text-lazyops-text">GitHub</h3>
            <p className="text-xs text-lazyops-muted">Install GitHub App, sync repos, and wire webhook delivery.</p>
          </Link>

          <div className="rounded-xl border border-dashed border-lazyops-border bg-lazyops-card/50 p-4">
            <h3 className="mb-1 text-sm font-semibold text-lazyops-text">More coming soon</h3>
            <p className="text-xs text-lazyops-muted">
              Additional providers are not implemented yet in backend APIs.
            </p>
          </div>
        </div>
      </SectionCard>
    </div>
  );
}
