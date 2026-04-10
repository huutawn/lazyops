'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { 
  Rocket, 
  ChevronRight, 
  Activity, 
  ShieldCheck, 
  Package, 
  Layers, 
  Clock, 
  Terminal, 
  Cpu, 
  AlertCircle,
  ArrowLeft
} from 'lucide-react';
import { useDeployment, useDeploymentAction } from '@/modules/deployments/deployment-hooks';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import type { BuildState, RolloutState } from '@/modules/deployments/deployment-types';
import type { LogEntry } from '@/modules/observability/observability-types';
import { listProjectLogs } from '@/modules/observability/observability-api';
import { useTrace } from '@/modules/observability/observability-hooks';
import { cn } from '@/lib/utils';
import { SectionCard } from '@/components/primitives/section-card';

const BUILD_STATE_VARIANT: Record<BuildState, 'success' | 'warning' | 'danger' | 'info' | 'neutral'> = {
  draft: 'neutral',
  queued: 'info',
  building: 'info',
  artifact_ready: 'success',
  planned: 'info',
  applying: 'warning',
  promoted: 'success',
  failed: 'danger',
  rolled_back: 'danger',
  superseded: 'neutral',
};

const ROLLOUT_STATE_VARIANT: Record<RolloutState, 'success' | 'warning' | 'danger' | 'info' | 'neutral'> = {
  queued: 'info',
  running: 'warning',
  candidate_ready: 'info',
  promoted: 'success',
  failed: 'danger',
  rolled_back: 'danger',
  canceled: 'neutral',
};

function formatState(state: string): string {
  return state.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

export default function DeploymentDetailPage() {
  const params = useParams();
  const projectId = params?.projectId as string | undefined;
  const deploymentId = params?.deploymentId as string;
  
  const { data, isLoading, isError } = useDeployment(projectId, deploymentId);
  const deploymentAction = useDeploymentAction(projectId, deploymentId);
  
  const revisionID = data?.revision_id;
  const deploymentLogs = useQuery({
    queryKey: ['deployment-runtime-logs', projectId, deploymentId, revisionID],
    queryFn: async (): Promise<LogEntry[]> => {
      if (!projectId || !revisionID) return [];
      const result = await listProjectLogs(projectId, { limit: 200 });
      if (result.error) throw new Error(result.error.message);
      const lines = result.data?.items ?? [];
      return lines.filter((line) => line.revision_id === revisionID).slice(0, 8);
    },
    enabled: !!projectId && !!revisionID,
    staleTime: 15 * 1000,
    refetchInterval: 10_1000,
  });

  const traceCorrelationID = deploymentLogs.data?.find((line) => line.correlation_id)?.correlation_id ?? '';
  const trace = useTrace(traceCorrelationID);

  if (isLoading) {
    return (
      <div className="max-w-[1400px] mx-auto py-10 lg:px-8">
        <SkeletonPage title cards={3} />
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="max-w-[1400px] mx-auto py-10 lg:px-8 flex flex-col gap-6">
        <Link href="/deployments" className="flex items-center gap-2 text-[#94a3b8] hover:text-white transition-colors mb-4">
          <ArrowLeft className="size-4" /> Quay lại danh sách
        </Link>
        <ErrorState title="Không tìm thấy bản triển khai" message={`Không thể tìm thấy thông tin cho ID: ${deploymentId}`} />
      </div>
    );
  }

  const dep = data;
  const isTerminal = ['promoted', 'failed', 'rolled_back', 'canceled'].includes(dep.rollout_state);
  const incident = dep.incident_summary;

  return (
    <div className="relative flex flex-col gap-8 max-w-[1400px] mx-auto py-10 lg:px-8">
      {/* Background Polish */}
      <div className="absolute -top-24 -right-24 size-[500px] rounded-full bg-[#0EA5E9]/5 blur-[120px] pointer-events-none" />
      
      <div className="relative z-10 flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2 text-[#94a3b8] text-sm font-medium mb-1">
            <Link href="/deployments" className="hover:text-white transition-colors">Triển khai</Link>
            <ChevronRight className="size-3" />
            <span className="text-[#38BDF8]">Revision {dep.revision}</span>
          </div>
          <h1 className="text-4xl font-extrabold tracking-tight text-white flex items-center gap-4">
            Bản triển khai #{dep.revision}
            {dep.promoted && <StatusBadge label="Đã phát hành" variant="success" size="lg" />}
          </h1>
          <p className="text-[#94a3b8] text-lg font-mono opacity-80">
            {dep.commit_sha} · {dep.runtime_mode}
          </p>
        </div>

        <div className="flex flex-wrap gap-3">
          {dep.can_cancel && (
            <button
              onClick={() => deploymentAction.mutate('cancel')}
              disabled={deploymentAction.isPending}
              className="rounded-xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-6 py-3 text-sm font-bold text-[#ef4444] transition-all hover:bg-[#ef4444]/20 disabled:opacity-50"
            >
              Hủy triển khai
            </button>
          )}
          {dep.can_promote && (
            <button
              onClick={() => deploymentAction.mutate('promote')}
              disabled={deploymentAction.isPending}
              className="rounded-xl bg-[#0EA5E9] px-6 py-3 text-sm font-bold text-white transition-all hover:bg-[#0284c7] hover:scale-105 active:scale-95 shadow-lg shadow-[#0ea5e9]/20"
            >
              Phát hành Production
            </button>
          )}
          {dep.can_rollback && (
            <button
              onClick={() => deploymentAction.mutate('rollback')}
              disabled={deploymentAction.isPending}
              className="rounded-xl border border-[#ef4444] bg-[#ef4444]/10 px-6 py-3 text-sm font-bold text-[#ef4444] transition-all hover:bg-[#ef4444]/20"
            >
              Rollback ngay
            </button>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Left Column - Core Info */}
        <div className="lg:col-span-2 flex flex-col gap-8">
          <SectionCard 
            title={<div className="flex items-center gap-2"><Activity className="size-5 text-[#38BDF8]" /> Trạng thái hiện tại</div>}
          >
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8 mb-8">
              <div className="flex flex-col gap-3">
                <span className="text-sm font-bold text-[#64748b] uppercase tracking-wider">Build</span>
                <div className="flex items-center gap-3">
                  <StatusBadge 
                    label={formatState(dep.build_state)} 
                    variant={BUILD_STATE_VARIANT[dep.build_state]} 
                    size="lg" 
                  />
                </div>
              </div>
              <div className="flex flex-col gap-3">
                <span className="text-sm font-bold text-[#64748b] uppercase tracking-wider">Rollout</span>
                <div className="flex items-center gap-3">
                  <StatusBadge 
                    label={formatState(dep.rollout_state)} 
                    variant={ROLLOUT_STATE_VARIANT[dep.rollout_state]} 
                    size="lg" 
                  />
                </div>
              </div>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-6 pt-6 border-t border-[#1e293b]">
              <SummaryField label="Hình thức" value={dep.trigger_kind} icon={<Rocket className="size-3.5" />} />
              <SummaryField label="Người kích hoạt" value={dep.triggered_by} icon={<Package className="size-3.5" />} />
              <SummaryField label="Bắt đầu" value={dep.started_at ? new Date(dep.started_at).toLocaleTimeString() : '—'} icon={<Clock className="size-3.5" />} />
              <SummaryField label="Kết thúc" value={dep.completed_at ? new Date(dep.completed_at).toLocaleTimeString() : '—'} icon={<Clock className="size-3.5" />} />
            </div>
          </SectionCard>

          <SectionCard 
            title={<div className="flex items-center gap-2"><Terminal className="size-5 text-[#38BDF8]" /> Nhật ký vận hành</div>}
            description="Các log mới nhất liên quan trực tiếp đến revision này."
          >
            <div className="rounded-xl border border-[#1e293b] bg-[#0B1120] p-4 min-h-[200px] font-mono shadow-inner">
              {deploymentLogs.isLoading ? (
                <p className="text-[#64748b] text-sm animate-pulse">Đang kết nối luồng log...</p>
              ) : deploymentLogs.data && deploymentLogs.data.length > 0 ? (
                <div className="flex flex-col gap-2">
                  {deploymentLogs.data.map((line) => (
                    <div key={line.id} className="text-[13px] leading-relaxed group">
                      <span className="text-[#64748b] mr-3">{new Date(line.timestamp).toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>
                      <span className={cn(
                        "font-bold mr-3",
                        line.level === 'error' ? 'text-[#ef4444]' : line.level === 'warn' ? 'text-[#eab308]' : 'text-[#10b981]'
                      )}>
                        [{line.level.toUpperCase()}]
                      </span>
                      <span className="text-[#e2e8f0] opacity-90 group-hover:opacity-100">{line.message}</span>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-12 opacity-40">
                  <Terminal className="size-10 text-[#64748b] mb-4" />
                  <p className="text-[#64748b] text-sm text-center">Chưa có log được ghi nhận cho revision này.</p>
                </div>
              )}
            </div>
          </SectionCard>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            <SectionCard title={<div className="flex items-center gap-2"><Layers className="size-5 text-[#38BDF8]" /> Dịch vụ</div>}>
              <div className="flex flex-col gap-3">
                {dep.services.map((svc) => (
                  <div key={svc.name} className="flex items-center justify-between rounded-xl bg-[#131c31] border border-[#1e293b] px-4 py-3">
                    <div className="flex flex-col">
                      <span className="text-[15px] font-bold text-white">{svc.name}</span>
                      <span className="text-xs text-[#64748b] font-mono">{svc.path}</span>
                    </div>
                    <StatusBadge label={svc.runtime_profile} variant="neutral" size="sm" dot={false} />
                  </div>
                ))}
              </div>
            </SectionCard>

            <SectionCard title={<div className="flex items-center gap-2"><Cpu className="size-5 text-[#38BDF8]" /> Phân bổ</div>}>
              <div className="flex flex-col gap-3">
                {dep.placement_assignments.map((pa) => (
                  <div key={pa.service_name} className="flex flex-col gap-1 rounded-xl bg-[#131c31] border border-[#1e293b] px-4 py-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-bold text-white">{pa.service_name}</span>
                      <span className="text-xs text-[#38BDF8] font-mono">{pa.target_id}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-[10px] text-[#64748b] uppercase font-bold tracking-widest">{pa.target_kind}</span>
                      <div className="flex gap-1">
                        {Object.entries(pa.labels).slice(0, 2).map(([k, v]) => (
                          <span key={k} className="text-[10px] text-[#94a3b8] bg-[#1e293b] px-1.5 py-0.5 rounded border border-[#334155]/30">
                            {k}: {v}
                          </span>
                        ))}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </SectionCard>
          </div>
        </div>

        {/* Right Column - Secondary Info */}
        <div className="flex flex-col gap-8">
          <SectionCard 
            title={<div className="flex items-center gap-2"><ShieldCheck className="size-5 text-[#38BDF8]" /> An toàn & Bảo mật</div>}
          >
            <div className="flex flex-col gap-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-[#94a3b8]">Tự động Rollback</span>
                <StatusBadge 
                  label={dep.safety_policy.auto_rollback_enabled ? 'Bật' : 'Tắt'} 
                  variant={dep.safety_policy.auto_rollback_enabled ? 'success' : 'warning'}
                  dot={false}
                />
              </div>
              <div className="flex flex-col gap-2">
                <span className="text-sm text-[#94a3b8]">Trình kích hoạt Rollback:</span>
                <div className="flex flex-wrap gap-2">
                  {dep.safety_policy.triggers.map((trigger) => (
                    <span key={trigger} className="text-[11px] text-white bg-[#1e293b] px-2 py-1 rounded-lg border border-[#334155]">{formatState(trigger)}</span>
                  ))}
                </div>
              </div>
              <p className="text-xs text-[#64748b] leading-relaxed italic">{dep.safety_policy.description}</p>
            </div>

            {incident && (
              <div className={cn(
                "mt-6 p-5 rounded-2xl border bg-opacity-10 backdrop-blur-sm",
                incident.state === 'healthy' ? 'border-[#10b981]/30 bg-[#10b981]' : 'border-[#ef4444]/30 bg-[#ef4444]'
              )}>
                <div className="flex items-center gap-2 mb-3">
                  <AlertCircle className={cn("size-5", incident.state === 'healthy' ? 'text-[#10b981]' : 'text-[#ef4444]')} />
                  <span className="text-[15px] font-bold text-white">{incident.headline}</span>
                </div>
                <p className="text-sm text-[#94a3b8] mb-4 leading-relaxed">{incident.reason}</p>
                {incident.primary_action && (
                  <Link
                    href={incident.primary_action.href}
                    className="flex items-center justify-center w-full rounded-xl bg-white/10 py-3 text-sm font-bold text-white transition-all hover:bg-white/20 border border-white/5"
                  >
                    {incident.primary_action.label}
                  </Link>
                )}
              </div>
            )}
          </SectionCard>

          <SectionCard title={<div className="flex items-center gap-2"><Clock className="size-5 text-[#38BDF8]" /> Lịch sử sự kiện</div>}>
            <div className="relative pl-6">
              <div className="absolute left-2 top-0 bottom-0 w-px bg-gradient-to-b from-[#334155] via-[#334155] to-transparent" />
              <div className="flex flex-col gap-8">
                {dep.timeline.map((event, i) => (
                  <div key={i} className="relative">
                    <div className={cn(
                      "absolute -left-[22px] top-1 size-3 rounded-full border-2 bg-[#0B1120] z-10",
                      event.state === 'failed' || event.state === 'rolled_back' ? 'border-[#ef4444]' :
                      event.state === 'promoted' ? 'border-[#10b981]' : 'border-[#334155]'
                    )} />
                    <div className="flex flex-col gap-1">
                      <span className="text-sm font-bold text-white">{event.label}</span>
                      <p className="text-xs text-[#64748b] leading-relaxed">{event.description}</p>
                      <span className="text-[10px] text-[#64748b] font-mono mt-1">
                        {new Date(event.timestamp).toLocaleTimeString()}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </SectionCard>

          <SectionCard title={<div className="flex items-center gap-2"><Activity className="size-5 text-[#38BDF8]" /> Giám sát Tracer</div>}>
            {traceCorrelationID ? (
              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-[#64748b] uppercase font-bold">Correlation ID</span>
                  <code className="text-[11px] text-[#38BDF8] bg-[#0B1120] p-2 rounded-lg border border-[#334155]/30 break-all">{traceCorrelationID}</code>
                </div>
                {trace.isLoading ? (
                  <div className="flex items-center gap-2 text-xs text-[#64748b]">
                    <div className="size-3 border-2 border-[#38BDF8] border-t-transparent rounded-full animate-spin" />
                    Đang phân tích vết...
                  </div>
                ) : trace.data ? (
                  <div className="space-y-4">
                    <div className="flex flex-col gap-1">
                      <span className="text-xs text-[#64748b] font-bold">Điểm nóng (Hotspot):</span>
                      <span className="text-sm font-bold text-[#ef4444]">{trace.data.latency_hotspot}</span>
                    </div>
                    <div className="flex flex-col gap-1">
                      <span className="text-xs text-[#64748b] font-bold">Tổng độ trễ:</span>
                      <span className="text-sm font-bold text-white">{trace.data.total_latency_ms} ms</span>
                    </div>
                    <Link
                      href="/observability"
                      className="flex items-center justify-center w-full rounded-xl bg-[#1e293b] py-3 text-xs font-bold text-[#94a3b8] transition-all hover:text-white border border-[#334155]"
                    >
                      Mở bảng giám sát &rarr;
                    </Link>
                  </div>
                ) : (
                  <p className="text-xs text-[#64748b]">Không có dữ liệu vết cho revision này.</p>
                )}
              </div>
            ) : (
              <p className="text-xs text-[#64748b]">Không tìm thấy correlation ID trong nhật ký.</p>
            )}
          </SectionCard>
        </div>
      </div>
    </div>
  );
}

function SummaryField({ label, value, icon }: { label: string; value: string; icon?: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2 text-[#64748b]">
        {icon}
        <span className="text-[11px] font-bold uppercase tracking-wider">{label}</span>
      </div>
      <span className="truncate text-sm font-bold text-white" title={value}>{value}</span>
    </div>
  );
}
