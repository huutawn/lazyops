'use client';

import { useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { ProjectTabs } from '@/components/primitives/project-tabs';
import { FormButton, FormField, FormInput } from '@/components/forms/form-fields';
import { useProjectRouting, useUpdateProjectRouting } from '@/modules/project-routing/project-routing-hooks';
import type { RoutingRoute } from '@/modules/project-routing/project-routing-types';

export default function RoutingPage() {
  const params = useParams();
  const projectId = params.projectId as string;

  const { data, isLoading, error } = useProjectRouting(projectId);
  const update = useUpdateProjectRouting(projectId);

  const [sharedDomain, setSharedDomain] = useState(data?.routing_policy?.shared_domain ?? '');
  const [routes, setRoutes] = useState<RoutingRoute[]>(
    data?.routing_policy?.routes ?? [],
  );
  const [isDirty, setIsDirty] = useState(false);

  const availableServices = useMemo(() => {
    return data?.available_services ?? [];
  }, [data?.available_services]);

  if (isLoading) {
    return <SkeletonPage title="Routing Configuration" cards={2} />;
  }

  if (error) {
    return (
      <ErrorState
        title="Failed to load routing configuration"
        message={error.message}
        actionLabel="Retry"
        onAction={() => window.location.reload()}
      />
    );
  }

  const handleAddRoute = () => {
    const newRoute: RoutingRoute = {
      path: '/',
      service: availableServices[0] ?? '',
      port: 0,
      websocket: false,
      strip_prefix: false,
    };
    setRoutes((prev) => [...prev, newRoute]);
    setIsDirty(true);
  };

  const handleRemoveRoute = (index: number) => {
    setRoutes((prev) => prev.filter((_, i) => i !== index));
    setIsDirty(true);
  };

  const handleRouteChange = (index: number, field: keyof RoutingRoute, value: string | number | boolean) => {
    setRoutes((prev) =>
      prev.map((route, i) => (i === index ? { ...route, [field]: value } : route)),
    );
    setIsDirty(true);
  };

  const handleSave = async () => {
    await update.mutateAsync({
      shared_domain: sharedDomain || undefined,
      routes,
    });
    setIsDirty(false);
  };

  const hasErrors = routes.some((r) => !r.service || !r.path);
  const hasOverlappingPaths = (() => {
    const nonRootPaths = routes.filter((r) => r.path !== '/').map((r) => r.path);
    for (let i = 0; i < nonRootPaths.length; i++) {
      for (let j = i + 1; j < nonRootPaths.length; j++) {
        if (nonRootPaths[i].startsWith(nonRootPaths[j]) || nonRootPaths[j].startsWith(nonRootPaths[i])) {
          return true;
        }
      }
    }
    return false;
  })();

  return (
    <div className="flex flex-col gap-6">
      <ProjectTabs projectId={projectId} />
      <PageHeader
        title="Định tuyến"
        subtitle="Cấu hình cách định tuyến lưu lượng bên ngoài đến các dịch vụ của bạn. Thiết lập định tuyến theo đường dẫn, WebSocket endpoint và shared domain."
      />

      <SectionCard
        title="Shared Domain"
        description="Configure a shared domain for path-based routing. All routes below will be served under this domain."
      >
        <FormField label="Shared Domain">
          <FormInput
            placeholder="app.yourdomain.com"
            value={sharedDomain}
            onChange={(e) => {
              setSharedDomain(e.target.value);
              setIsDirty(true);
            }}
          />
        </FormField>
        <p className="mt-3 text-sm text-[#94a3b8]">
          Example: <code className="rounded bg-[#0B1120]/60 px-1.5 py-0.5 text-[#38BDF8]">app.project.sslip.io</code>.
          Routes like <code className="rounded bg-[#0B1120]/60 px-1.5 py-0.5 text-[#38BDF8]">/api</code> and <code className="rounded bg-[#0B1120]/60 px-1.5 py-0.5 text-[#38BDF8]">/</code> will be mapped to different services.
        </p>
      </SectionCard>

      <SectionCard
        title="Routes"
        description="Define path prefixes and map them to backend services."
        actions={
          <button
            type="button"
            onClick={handleAddRoute}
            className="rounded-lg border border-[#1e293b] bg-[#0B1120]/40 px-4 py-2 text-sm font-medium text-[#94a3b8] transition-colors hover:border-[#0EA5E9]/50 hover:text-white"
          >
            + Add Route
          </button>
        }
      >
        {routes.length === 0 ? (
          <div className="rounded-xl border border-dashed border-[#1e293b] p-8 text-center">
            <p className="text-sm text-[#94a3b8]">
              No routes configured. Click &quot;Add Route&quot; to enable path-based routing.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {hasOverlappingPaths && (
              <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 px-4 py-3 text-sm text-amber-400">
                <strong>Warning:</strong> Some path prefixes overlap. This may cause routing conflicts.
              </div>
            )}

            {routes.map((route, index) => (
              <div
                key={index}
                className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-4 space-y-4"
              >
                <div className="flex items-center justify-between">
                  <span className="text-sm font-semibold text-white">Route #{index + 1}</span>
                  <button
                    type="button"
                    onClick={() => handleRemoveRoute(index)}
                    className="text-sm text-[#ef4444] hover:underline"
                  >
                    Remove
                  </button>
                </div>

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
                  <FormField label="Path Prefix">
                    <FormInput
                      placeholder="/api"
                      value={route.path}
                      onChange={(e) =>
                        handleRouteChange(index, 'path', e.target.value)
                      }
                    />
                  </FormField>

                  <FormField label="Service">
                    <select
                      value={route.service}
                      onChange={(e) =>
                        handleRouteChange(index, 'service', e.target.value)
                      }
                      className="flex h-12 w-full rounded-xl border border-[#1e293b] bg-[#0B1120]/40 px-4 text-[15px] text-white outline-none transition-all focus:border-[#0EA5E9]/50 focus:ring-4 focus:ring-[#0EA5E9]/10"
                    >
                      <option value="">Select service...</option>
                      {availableServices.map((svc) => (
                        <option key={svc} value={svc}>
                          {svc}
                        </option>
                      ))}
                    </select>
                  </FormField>

                  <FormField label="Port">
                    <FormInput
                      type="number"
                      placeholder="Auto"
                      value={route.port || ''}
                      onChange={(e) =>
                        handleRouteChange(index, 'port', parseInt(e.target.value) || 0)
                      }
                    />
                  </FormField>

                  <div className="flex items-center gap-4 pt-8">
                    <label className="flex items-center gap-2 text-sm text-[#94a3b8]">
                      <input
                        type="checkbox"
                        checked={route.websocket ?? false}
                        onChange={(e) =>
                          handleRouteChange(index, 'websocket', e.target.checked)
                        }
                        className="h-4 w-4 rounded border-[#1e293b] bg-[#0B1120]/40 text-[#0EA5E9] focus:ring-[#0EA5E9]/30"
                      />
                      WebSocket
                    </label>
                  </div>

                  <div className="flex items-center gap-4 pt-8">
                    <label className="flex items-center gap-2 text-sm text-[#94a3b8]">
                      <input
                        type="checkbox"
                        checked={route.strip_prefix ?? false}
                        onChange={(e) =>
                          handleRouteChange(index, 'strip_prefix', e.target.checked)
                        }
                        className="h-4 w-4 rounded border-[#1e293b] bg-[#0B1120]/40 text-[#0EA5E9] focus:ring-[#0EA5E9]/30"
                      />
                      Strip Prefix
                    </label>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </SectionCard>

      <SectionCard title="Caddyfile Preview" description="Preview of the generated Caddy configuration">
        <pre className="overflow-x-auto rounded-xl bg-[#0B1120]/60 p-4 text-xs font-mono text-[#94a3b8]">
          {generateCaddyfilePreview(sharedDomain, routes)}
        </pre>
      </SectionCard>

      <div className="flex justify-end">
        <div className="w-64">
          <FormButton
            type="button"
            onClick={handleSave}
            disabled={!isDirty || hasErrors || hasOverlappingPaths || update.isPending}
            loading={update.isPending}
          >
            {isDirty ? 'Save Changes' : 'No Changes'}
          </FormButton>
        </div>
      </div>
    </div>
  );
}

function generateCaddyfilePreview(sharedDomain: string, routes: RoutingRoute[]): string {
  if (!sharedDomain || routes.length === 0) {
    return '# No shared domain or routes configured\n# Add a shared domain and routes above to see the Caddyfile preview';
  }

  let caddyfile = `{\n  auto_https on\n}\n\n${sharedDomain} {\n  encode zstd gzip\n\n`;

  // WebSocket routes first
  const wsRoutes = routes.filter((r) => r.websocket);
  wsRoutes.forEach((route, i) => {
    caddyfile += `  @ws${i} path ${route.path}*\n`;
    caddyfile += `  handle @ws${i} {\n`;
    caddyfile += `    reverse_proxy ${route.service}:${route.port || '<port>'} {\n`;
    caddyfile += `      transport http {\n`;
    caddyfile += `        keepalive 60s\n`;
    caddyfile += `      }\n`;
    caddyfile += `    }\n`;
    caddyfile += `  }\n\n`;
  });

  // HTTP routes
  routes.filter((r) => !r.websocket).forEach((route) => {
    if (route.path === '/') {
      caddyfile += `  handle {\n`;
      if (route.strip_prefix) {
        caddyfile += `    uri strip_prefix /\n`;
      }
      caddyfile += `    reverse_proxy ${route.service}:${route.port || '<port>'}\n`;
      caddyfile += `  }\n\n`;
    } else {
      const matcher = route.path.replace(/\//g, '_').replace(/^-/, '');
      caddyfile += `  @${matcher} path ${route.path}*\n`;
      caddyfile += `  handle @${matcher} {\n`;
      if (route.strip_prefix) {
        caddyfile += `    uri strip_prefix ${route.path}\n`;
      }
      caddyfile += `    reverse_proxy ${route.service}:${route.port || '<port>'}\n`;
      caddyfile += `  }\n\n`;
    }
  });

  caddyfile += `}\n`;
  return caddyfile;
}
