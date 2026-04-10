'use client';

import Link from 'next/link';
import { ArrowLeft, CheckCircle2, GitCommit, Server, Clock } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useParams } from 'next/navigation';

export default function DeploymentDetailsPage() {
  const params = useParams();
  const id = params.id || 'd1';

  return (
    <div className="flex flex-col max-w-4xl mx-auto py-8 lg:px-4">
      <div className="flex items-start justify-between mb-8">
        <div className="flex items-start gap-4">
          <Link href="/projects" className="mt-1.5 text-white hover:text-[#0EA5E9] transition-colors">
            <ArrowLeft className="size-5" />
          </Link>
          <div>
            <h1 className="text-2xl font-bold text-white mb-1">Chi tiết triển khai</h1>
            <p className="text-[#94a3b8] text-[15px]">my-saas-app • #{id}</p>
          </div>
        </div>
        <div className="flex items-center gap-1.5 rounded-full border border-[#10b981]/30 bg-[#10b981]/10 px-3 py-1.5 text-[13px] font-medium text-[#10b981]">
          <CheckCircle2 className="size-4" />
          Thành công
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
        <div className="flex items-center gap-4 rounded-xl border border-[#1e293b] bg-[#0F172A] p-4">
          <div className="text-[#64748b]">
            <GitCommit className="size-5" />
          </div>
          <div>
            <div className="text-[12px] text-[#64748b] font-medium mb-0.5">Commit</div>
            <div className="text-[14px] font-semibold text-white">fix: update login</div>
          </div>
        </div>

        <div className="flex items-center gap-4 rounded-xl border border-[#1e293b] bg-[#0F172A] p-4">
          <div className="text-[#64748b]">
            <Server className="size-5" />
          </div>
          <div>
            <div className="text-[12px] text-[#64748b] font-medium mb-0.5">Máy chủ</div>
            <div className="text-[14px] font-semibold text-white">192.168.1.100</div>
          </div>
        </div>

        <div className="flex items-center gap-4 rounded-xl border border-[#1e293b] bg-[#0F172A] p-4">
          <div className="text-[#64748b]">
            <Clock className="size-5" />
          </div>
          <div>
            <div className="text-[12px] text-[#64748b] font-medium mb-0.5">Thời gian</div>
            <div className="text-[14px] font-semibold text-white">49 giây</div>
          </div>
        </div>
      </div>

      <div className="mb-8">
        <h2 className="text-[17px] font-bold text-white mb-4">Tiến trình</h2>
        <div className="rounded-xl border border-[#1e293b] bg-[#0F172A] p-6">
          <div className="flex flex-col gap-5">
            {[
              { time: '14:32:01', label: 'Bắt đầu triển khai' },
              { time: '14:32:03', label: 'Kéo mã nguồn từ GitHub' },
              { time: '14:32:15', label: 'Build ứng dụng' },
              { time: '14:32:45', label: 'Khởi động container' },
              { time: '14:32:48', label: 'Health check' },
              { time: '14:32:50', label: 'Hoàn tất' },
            ].map((step, i) => (
              <div key={i} className="flex items-center gap-6">
                <div className="text-[13px] text-[#64748b] font-mono w-16 text-right">
                  {step.time}
                </div>
                <div className="size-2 rounded-full bg-[#10b981] shadow-[0_0_8px_rgba(16,185,129,0.6)]" />
                <div className="text-[14px] font-medium text-white">
                  {step.label}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div>
        <h2 className="text-[17px] font-bold text-white mb-4">Nhật ký</h2>
        <div className="rounded-xl border border-[#1e293b] bg-[#0F172A] overflow-hidden p-1">
          <div className="bg-[#090E17] rounded-lg p-5 font-mono text-[13px] leading-relaxed text-[#94a3b8] overflow-x-auto h-[250px] overflow-y-auto">
            <div>[14:32:01] Starting deployment...</div>
            <div>[14:32:03] Cloning repo user/my-saas-app@main</div>
            <div>[14:32:10] Installing dependencies...</div>
            <div>[14:32:15] Building application...</div>
            <div>[14:32:40] Build completed successfully</div>
            <div>[14:32:42] Creating container...</div>
            <div>[14:32:45] Container started on port 3000</div>
          </div>
        </div>
      </div>
    </div>
  );
}
