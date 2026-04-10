'use client';

import Link from 'next/link';
import { Rocket, FolderGit2, ServerIcon, CheckCircle2, Loader2, XCircle } from 'lucide-react';
import { useSession } from '@/lib/auth/auth-hooks';
import { SkeletonPage } from '@/components/primitives/skeleton';

export default function DashboardPage() {
  const { data: session, isLoading } = useSession();

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  return (
    <div className="flex flex-col gap-8 max-w-5xl lg:px-4 py-4">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white mb-1 flex items-center gap-2">
            Xin chào 👋
          </h1>
          <p className="text-[#94a3b8] text-[15px]">
            Tổng quan hoạt động của bạn
          </p>
        </div>
        <Link
          href="/onboarding"
          className="rounded-lg bg-[#0EA5E9] px-4 py-2 text-[14px] font-semibold text-white shadow-sm transition-all hover:bg-[#0284c7] flex items-center gap-2"
        >
          <span>+</span>
          Dự án mới
        </Link>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="flex items-center gap-4 rounded-xl border border-[#1e293b] bg-[#0F172A] p-5 shadow-sm">
          <div className="flex size-12 items-center justify-center rounded-lg bg-[#0E3B4D] text-[#38BDF8]">
            <FolderGit2 className="size-6" />
          </div>
          <div>
            <div className="text-2xl font-bold text-white leading-none mb-1">3</div>
            <div className="text-[13px] text-[#94a3b8] font-medium">Dự án</div>
          </div>
        </div>

        <div className="flex items-center gap-4 rounded-xl border border-[#1e293b] bg-[#0F172A] p-5 shadow-sm">
          <div className="flex size-12 items-center justify-center rounded-lg bg-[#0E3B4D] text-[#38BDF8]">
            <ServerIcon className="size-6" />
          </div>
          <div>
            <div className="text-2xl font-bold text-white leading-none mb-1">2</div>
            <div className="text-[13px] text-[#94a3b8] font-medium">Máy chủ</div>
          </div>
        </div>

        <div className="flex items-center gap-4 rounded-xl border border-[#1e293b] bg-[#0F172A] p-5 shadow-sm">
          <div className="flex size-12 items-center justify-center rounded-lg bg-[#0E3B4D] text-[#38BDF8]">
            <Rocket className="size-6" />
          </div>
          <div>
            <div className="text-2xl font-bold text-white leading-none mb-1">12</div>
            <div className="text-[13px] text-[#94a3b8] font-medium">Lần triển khai</div>
          </div>
        </div>
      </div>

      <div className="mt-2">
        <h2 className="text-[17px] font-bold text-white mb-4">Triển khai gần đây</h2>
        
        <div className="flex flex-col rounded-xl border border-[#1e293b] bg-[#0F172A] overflow-hidden">
          <div className="flex items-center justify-between p-4 border-b border-[#1e293b] hover:bg-[#131c31] transition-colors">
            <div className="flex items-center gap-3">
              <Rocket className="size/[18px] text-[#64748b]" />
              <span className="text-[15px] font-semibold text-white">my-saas-app</span>
            </div>
            <div className="flex items-center gap-6">
              <span className="text-[13px] text-[#64748b]">2 phút trước</span>
              <div className="flex items-center gap-1.5 rounded-full border border-[#10b981]/30 bg-[#10b981]/10 px-2.5 py-1 text-[12px] font-medium text-[#10b981]">
                <CheckCircle2 className="size-3.5" />
                Thành công
              </div>
            </div>
          </div>

          <div className="flex items-center justify-between p-4 border-b border-[#1e293b] hover:bg-[#131c31] transition-colors">
            <div className="flex items-center gap-3">
              <Rocket className="size/[18px] text-[#64748b]" />
              <span className="text-[15px] font-semibold text-white">landing-page</span>
            </div>
            <div className="flex items-center gap-6">
              <span className="text-[13px] text-[#64748b]">5 phút trước</span>
              <div className="flex items-center gap-1.5 rounded-full border border-[#0ea5e9]/30 bg-[#0ea5e9]/10 px-2.5 py-1 text-[12px] font-medium text-[#0ea5e9]">
                <Loader2 className="size-3.5 animate-spin" />
                Đang chạy
              </div>
            </div>
          </div>

          <div className="flex items-center justify-between p-4 hover:bg-[#131c31] transition-colors">
            <div className="flex items-center gap-3">
              <Rocket className="size/[18px] text-[#64748b]" />
              <span className="text-[15px] font-semibold text-white">api-service</span>
            </div>
            <div className="flex items-center gap-6">
              <span className="text-[13px] text-[#64748b]">1 giờ trước</span>
              <div className="flex items-center gap-1.5 rounded-full border border-[#ef4444]/30 bg-[#ef4444]/10 px-2.5 py-1 text-[12px] font-medium text-[#ef4444]">
                <XCircle className="size-3.5" />
                Lỗi
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
