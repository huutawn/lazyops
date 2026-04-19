'use client';

import { type ReactNode } from 'react';
import { Layers, Server, Database } from 'lucide-react';
import { CreateProjectForm } from '@/modules/onboarding/create-project-form';

export default function NewProjectPage() {
  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-8 px-4 py-10 lg:px-8">
      <div className="grid gap-6 lg:grid-cols-[0.85fr_1.15fr]">
        <section className="rounded-[2rem] border border-[#1e293b] bg-[#07111f] p-8">
          <div className="max-w-xl">
            <div className="mb-5 inline-flex rounded-full border border-[#0EA5E9]/30 bg-[#0EA5E9]/10 px-4 py-2 text-xs font-bold uppercase tracking-[0.18em] text-[#38BDF8]">
              Service-first project creation
            </div>
            <h1 className="text-4xl font-black tracking-tight text-white">
              Project chi la namespace.
              <br />
              Services moi la don vi deploy.
            </h1>
            <p className="mt-5 text-base leading-relaxed text-[#94a3b8]">
              Khoi tao project moi se scaffold thang unified service inventory, san sang cho case `backend + postgres + frontend`
              ngay tu lan tao dau tien.
            </p>
          </div>

          <div className="mt-8 grid gap-4">
            <FeatureCard
              icon={<Server className="size-5" />}
              title="Repo services"
              description="Tao API/backend va frontend rieng, giu path/name ro rang de deploy theo service."
            />
            <FeatureCard
              icon={<Database className="size-5" />}
              title="Internal Postgres"
              description="Nhap ten env can map, LazyOps tu inject DB runtime values vao dung field tren VPS/K3s."
            />
            <FeatureCard
              icon={<Layers className="size-5" />}
              title="Unified source of truth"
              description="Khong con bootstrap duong vong qua internal-services lane cu khi khoi tao project moi."
            />
          </div>
        </section>

        <section className="rounded-[2rem] border border-[#1e293b] bg-[#020817]/90 p-6 sm:p-8">
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
    <div className="rounded-3xl border border-[#1e293b] bg-[#0B1120]/70 p-5">
      <div className="mb-3 inline-flex rounded-2xl bg-[#0EA5E9]/10 p-3 text-[#38BDF8]">{icon}</div>
      <h2 className="text-base font-bold text-white">{title}</h2>
      <p className="mt-2 text-sm leading-relaxed text-[#94a3b8]">{description}</p>
    </div>
  );
}
