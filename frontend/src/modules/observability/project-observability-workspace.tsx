'use client';

import { useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { ObservabilityConsole } from '@/modules/observability/observability-console';
import {
  PROJECT_OBSERVABILITY_TABS,
  resolveProjectObservabilityTab,
  type ProjectObservabilityTab,
} from '@/modules/observability/project-observability-tabs';
import { ProjectRuntimeWorkspace } from '@/modules/project-runtime/project-runtime-workspace';

type ProjectObservabilityWorkspaceProps = {
  projectId: string;
};

export function ProjectObservabilityWorkspace({ projectId }: ProjectObservabilityWorkspaceProps) {
  const searchParams = useSearchParams();
  const [activeTab, setActiveTab] = useState<ProjectObservabilityTab>(() =>
    resolveProjectObservabilityTab({
      tab: searchParams.get('tab'),
      service: searchParams.get('service'),
    }),
  );

  useEffect(() => {
    setActiveTab(
      resolveProjectObservabilityTab({
        tab: searchParams.get('tab'),
        service: searchParams.get('service'),
      }),
    );
  }, [searchParams]);

  return (
    <div className="flex flex-col gap-6">
      <nav className="flex gap-2 overflow-x-auto border-b border-[#1e293b] pb-2">
        {PROJECT_OBSERVABILITY_TABS.map((tab) => {
          const isActive = tab.key === activeTab;
          return (
            <button
              key={tab.key}
              type="button"
              onClick={() => setActiveTab(tab.key)}
              className={
                isActive
                  ? 'whitespace-nowrap rounded-2xl bg-[#0EA5E9]/12 px-5 py-3 text-base font-bold text-[#38bdf8] shadow-[0_10px_30px_rgba(14,165,233,0.14)]'
                  : 'whitespace-nowrap rounded-2xl px-5 py-3 text-base font-bold text-[#94a3b8] transition-colors hover:bg-[#0B1120]/70 hover:text-white'
              }
            >
              {tab.label}
            </button>
          );
        })}
      </nav>

      {activeTab === 'monitoring' ? (
        <ObservabilityConsole
          fixedProjectId={projectId}
          title="Nhật ký / Monitoring"
          subtitle="Logs, metrics, traces, and incidents for this project."
        />
      ) : (
        <ProjectRuntimeWorkspace projectId={projectId} />
      )}
    </div>
  );
}
