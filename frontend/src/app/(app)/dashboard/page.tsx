'use client';

import Link from 'next/link';
import { Rocket, FolderGit2, ServerIcon, CheckCircle2, Loader2, XCircle, AlertCircle } from 'lucide-react';
import { useSession } from '@/lib/auth/auth-hooks';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { useProjects } from '@/modules/projects/project-hooks';
import { useDeployments } from '@/modules/deployments/deployment-hooks';
import { useInstances } from '@/modules/instances/instance-hooks';
import { cn } from '@/lib/utils';

export default function DashboardPage() {
  const { data: session, isLoading: sessionLoading } = useSession();
  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const { data: deploymentsData, isLoading: deploymentsLoading } = useDeployments();
  const { data: instancesData, isLoading: instancesLoading } = useInstances();

  const isLoading = sessionLoading || projectsLoading || deploymentsLoading || instancesLoading;

  if (isLoading) {
    return (
      <div className="max-w-[1400px] mx-auto py-8 lg:px-8">
        <SkeletonPage title cards={3} />
      </div>
    );
  }

  const projectsCount = projectsData?.items?.length ?? 0;
  const serversCount = instancesData?.items?.length ?? 0;
  const deployments = deploymentsData?.items ?? [];
  const deploymentsCount = deployments.length;
  const recentDeployments = deployments.slice(0, 5);

  const formatDistanceToNow = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

    if (diffInSeconds < 60) return `${diffInSeconds} giây trước`;
    const diffInMinutes = Math.floor(diffInSeconds / 60);
    if (diffInMinutes < 60) return `${diffInMinutes} phút trước`;
    const diffInHours = Math.floor(diffInMinutes / 60);
    if (diffInHours < 24) return `${diffInHours} giờ trước`;
    const diffInDays = Math.floor(diffInHours / 24);
    return `${diffInDays} ngày trước`;
  };

  const getStatusBadge = (state: string) => {
    switch (state) {
      case 'promoted':
        return (
          <div className="flex items-center gap-2 rounded-full border border-[#10b981]/30 bg-[#10b981]/10 px-3 py-1 text-[13px] font-medium text-[#10b981]">
            <CheckCircle2 className="size-4" />
            Thành công
          </div>
        );
      case 'running':
      case 'building':
      case 'applying':
        return (
          <div className="flex items-center gap-2 rounded-full border border-[#0ea5e9]/30 bg-[#0ea5e9]/10 px-3 py-1 text-[13px] font-medium text-[#0ea5e9]">
            <Loader2 className="size-4 animate-spin" />
            Đang chạy
          </div>
        );
      case 'failed':
      case 'rolled_back':
        return (
          <div className="flex items-center gap-2 rounded-full border border-[#ef4444]/30 bg-[#ef4444]/10 px-3 py-1 text-[13px] font-medium text-[#ef4444]">
            <XCircle className="size-4" />
            Lỗi
          </div>
        );
      default:
        return (
          <div className="flex items-center gap-2 rounded-full border border-[#64748b]/30 bg-[#64748b]/10 px-3 py-1 text-[13px] font-medium text-[#64748b]">
            <AlertCircle className="size-4" />
            {state}
          </div>
        );
    }
  };

  return (
    <div className="relative flex flex-col gap-10 max-w-[1400px] mx-auto py-10 lg:px-8 overflow-hidden">
      {/* Subtle Background Glow */}
      <div className="absolute -top-24 -right-24 size-[500px] rounded-full bg-[#0EA5E9]/5 blur-[120px] pointer-events-none" />
      <div className="absolute top-1/2 -left-24 size-[400px] rounded-full bg-[#38BDF8]/5 blur-[100px] pointer-events-none" />

      <div className="relative z-10 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-6">
        <div>
          <h1 className="text-4xl font-extrabold tracking-tight text-white mb-2 flex items-center gap-3">
            Xin chào {session?.display_name || 'bạn'} 👋
          </h1>
          <p className="text-[#94a3b8] text-lg font-medium opacity-90">
            Tổng quan hoạt động của bạn
          </p>
        </div>
        <Link
          href="/onboarding"
          className="rounded-xl bg-gradient-to-r from-[#0EA5E9] to-[#38BDF8] px-7 py-4 text-base font-bold text-white shadow-[0_10px_30px_rgba(14,165,233,0.3)] transition-all hover:scale-105 hover:brightness-110 active:scale-95 flex items-center gap-3"
        >
          <span className="text-xl">+</span>
          Dự án mới
        </Link>
      </div>

      <div className="relative z-10 grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="group relative flex items-center gap-6 rounded-2xl border border-[#1e293b] bg-[#0F172A]/80 backdrop-blur-md p-8 shadow-2xl transition-all hover:border-[#38BDF8]/40 hover:bg-[#0F172A]">
          <div className="flex size-16 items-center justify-center rounded-xl bg-gradient-to-br from-[#0E3B4D] to-[#072a38] text-[#38BDF8] shadow-inner">
            <FolderGit2 className="size-8" />
          </div>
          <div>
            <div className="text-4xl font-black text-white leading-tight mb-1 group-hover:scale-110 transition-transform origin-left">{projectsCount}</div>
            <div className="text-sm text-[#64748b] font-bold tracking-[0.1em] uppercase opacity-80">Dự án</div>
          </div>
          {/* Subtle Card Glow */}
          <div className="absolute inset-0 rounded-2xl bg-gradient-to-br from-[#38BDF8]/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
        </div>

        <div className="group relative flex items-center gap-6 rounded-2xl border border-[#1e293b] bg-[#0F172A]/80 backdrop-blur-md p-8 shadow-2xl transition-all hover:border-[#38BDF8]/40 hover:bg-[#0F172A]">
          <div className="flex size-16 items-center justify-center rounded-xl bg-gradient-to-br from-[#0E3B4D] to-[#072a38] text-[#38BDF8] shadow-inner">
            <ServerIcon className="size-8" />
          </div>
          <div>
            <div className="text-4xl font-black text-white leading-tight mb-1 group-hover:scale-110 transition-transform origin-left">{serversCount}</div>
            <div className="text-sm text-[#64748b] font-bold tracking-[0.1em] uppercase opacity-80">Máy chủ</div>
          </div>
          <div className="absolute inset-0 rounded-2xl bg-gradient-to-br from-[#38BDF8]/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
        </div>

        <div className="group relative flex items-center gap-6 rounded-2xl border border-[#1e293b] bg-[#0F172A]/80 backdrop-blur-md p-8 shadow-2xl transition-all hover:border-[#38BDF8]/40 hover:bg-[#0F172A]">
          <div className="flex size-16 items-center justify-center rounded-xl bg-gradient-to-br from-[#0E3B4D] to-[#072a38] text-[#38BDF8] shadow-inner">
            <Rocket className="size-8" />
          </div>
          <div>
            <div className="text-4xl font-black text-white leading-tight mb-1 group-hover:scale-110 transition-transform origin-left">{deploymentsCount}</div>
            <div className="text-sm text-[#64748b] font-bold tracking-[0.1em] uppercase opacity-80">Triển khai</div>
          </div>
          <div className="absolute inset-0 rounded-2xl bg-gradient-to-br from-[#38BDF8]/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
        </div>
      </div>

      <div className="relative z-10 mt-4 flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <h2 className="text-2xl font-bold text-white tracking-tight">Triển khai gần đây</h2>
          <Link href="/deployments" className="text-[#38BDF8] hover:text-[#0EA5E9] font-bold transition-colors flex items-center gap-2">
            Xem tất cả <span className="text-lg">&rarr;</span>
          </Link>
        </div>
        
        <div className="flex flex-col rounded-2xl border border-[#1e293b] bg-[#0F172A]/60 backdrop-blur-xl overflow-hidden shadow-2xl">
          {recentDeployments.length > 0 ? (
            recentDeployments.map((dep, index) => (
              <Link
                key={dep.id}
                href={`/deployments/${dep.id}`}
                className={cn(
                  "flex items-center justify-between p-7 transition-all hover:bg-[#131c31]/80",
                  index !== recentDeployments.length - 1 && "border-b border-[#1e293b]/60"
                )}
              >
                <div className="flex items-center gap-6">
                  <div className="flex size-12 items-center justify-center rounded-xl bg-[#1e293b] text-[#64748b] shadow-inner group-hover:text-white transition-colors">
                    <Rocket className="size-6" />
                  </div>
                  <div className="flex flex-col gap-1">
                    <span className="text-xl font-bold text-white tracking-tight">
                      Revision: {dep.revision}
                    </span>
                    <span className="text-sm text-[#64748b] font-mono opacity-80">
                      {dep.commit_sha.slice(0, 7)}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-12">
                  <span className="text-sm text-[#64748b] font-bold italic opacity-70">{formatDistanceToNow(dep.created_at)}</span>
                  {getStatusBadge(dep.rollout_state)}
                </div>
              </Link>
            ))
          ) : (
            <div className="p-20 text-center">
              <div className="size-20 bg-[#1e293b] rounded-full flex items-center justify-center mx-auto mb-6 opacity-40">
                <Rocket className="size-10 text-[#64748b]" />
              </div>
              <p className="text-[#64748b] text-xl font-medium">Chưa có dữ liệu triển khai nào.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
