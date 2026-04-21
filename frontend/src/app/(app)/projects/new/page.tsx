'use client';

import { type ReactNode } from 'react';
import { Layers, Server, Database } from 'lucide-react';
import { CreateProjectForm } from '@/modules/onboarding/create-project-form';

export default function NewProjectPage() {
  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-8 px-6 py-10 lg:px-8">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
        <section className="min-w-0 rounded-[2rem] border border-[#1e293b] bg-[#07111f] p-8">
          <div className="max-w-xl">
            <div className="mb-5 inline-flex rounded-full border border-[#0EA5E9]/30 bg-[#0EA5E9]/10 px-6 py-2 text-sm font-bold uppercase tracking-[0.18em] text-[#38BDF8]">
              Tạo Project Mới
            </div>
            <h1 className="text-4xl lg:text-5xl font-black tracking-tight text-white">
              Project là không gian làm việc.<br />
              Service là đơn vị Deploy.
            </h1>
            <p className="mt-5 text-base leading-relaxed text-[#94a3b8]">
              Tạo một Project mới để bắt đầu. Bạn có thể thêm mã nguồn, cấu hình Repository và các Database (Managed Services) tại bước tiếp theo.
            </p>
          </div>

          <div className="mt-8 grid gap-4">
            <FeatureCard
              icon={<Server className="size-5" />}
              title="Cấu hình Service dễ dàng"
              description="Thêm API, frontend, worker hoặc bất kỳ ứng dụng nào từ Repository chỉ với vài cú click."
            />
            <FeatureCard
              icon={<Database className="size-5" />}
              title="Tích hợp Database"
              description="Hỗ trợ các nền tảng phổ biến như PostgreSQL, MySQL, Redis..."
            />
            <FeatureCard
              icon={<Layers className="size-5" />}
              title="Quản lý tập trung"
              description="Tất cả các thành phần của Project được quản trị xuyên suốt trên cùng một giao diện hiển thị rõ ràng."
            />
          </div>
        </section>

        <section className="min-w-0 rounded-[2rem] border border-[#1e293b] bg-[#020817]/90 p-6 sm:p-8">
          <CreateProjectForm />
        </section>
      </div>
    </div>
  );
}

function FeatureCard({
  icon,
  title,
  description,
}: {
  icon: ReactNode;
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-3xl border border-[#1e293b] bg-[#0B1120]/70 p-6">
      <div className="mb-3 inline-flex rounded-2xl bg-[#0EA5E9]/10 p-3 text-[#38BDF8]">{icon}</div>
      <h2 className="text-base font-bold text-white">{title}</h2>
      <p className="mt-2 text-base leading-relaxed text-[#94a3b8]">{description}</p>
    </div>
  );
}
