'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Check, Github, Server, Database, ServerCrash, Layers, DatabaseZap, Box, TerminalSquare, Rocket, PlaySquare } from 'lucide-react';
import { cn } from '@/lib/utils';

export default function NewProjectWizard() {
  const router = useRouter();
  const [step, setStep] = useState(1);

  // Step 1 state
  const [selectedRepo, setSelectedRepo] = useState<string | null>(null);

  // Step 2 state
  const [ip, setIp] = useState('192.168.1.100');
  const [sshUser, setSshUser] = useState('root');

  // Step 3 state
  const [services, setServices] = useState({
    postgresql: false,
    mysql: false,
    redis: false,
    rabbitmq: false,
    kafka: false,
  });

  const STEPS = [
    { id: 1, title: 'Mã nguồn', desc: 'Kết nối\nGitHub' },
    { id: 2, title: 'Máy chủ', desc: 'Kết nối\nVPS' },
    { id: 3, title: 'Dịch vụ', desc: 'Chọn\nservice' },
    { id: 4, title: 'Triển khai', desc: 'Bấm\ndeploy' },
  ];

  return (
    <div className="flex flex-col max-w-4xl mx-auto py-8 lg:px-4">
      <div className="text-center mb-10 text-white">
        <h1 className="text-3xl font-bold mb-2">Tạo dự án mới</h1>
        <p className="text-[#94a3b8]">Hoàn tất 4 bước đơn giản để triển khai ứng dụng</p>
      </div>

      <div className="flex items-center justify-center mb-12">
        {STEPS.map((s, idx) => {
          const isActive = step === s.id;
          const isCompleted = step > s.id;

          return (
            <div key={s.id} className="flex items-center">
              <div className="flex flex-col items-center relative text-center w-24">
                <div
                  className={cn(
                    "flex size-10 items-center justify-center rounded-full font-bold text-[15px] z-10 transition-colors duration-300",
                    isActive ? "bg-[#0EA5E9] text-white shadow-[0_0_15px_rgba(14,165,233,0.5)]" :
                    isCompleted ? "bg-[#10B981] text-white" :
                    "bg-[#1e293b] text-[#64748b]"
                  )}
                >
                  {isCompleted ? <Check className="size-5" /> : s.id}
                </div>
                <div className="mt-3 leading-tight">
                  <div className={cn("text-[14px] font-bold", isActive || isCompleted ? "text-white" : "text-[#64748b]")}>
                    {s.title}
                  </div>
                  <div className="text-[12px] text-[#64748b] whitespace-pre-line mt-1">
                    {s.desc}
                  </div>
                </div>
              </div>

              {idx < STEPS.length - 1 && (
                <div className="w-16 sm:w-24 h-px -mt-10 bg-[#1e293b]" />
              )}
            </div>
          );
        })}
      </div>

      <div className="bg-[#0B1120] border border-[#1e293b] rounded-2xl shadow-xl flex flex-col p-6 max-w-3xl mx-auto w-full">
        {step === 1 && (
          <div className="animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div className="flex items-center gap-3 mb-2">
              <div className="text-[#38BDF8]">
                <Github className="size-6" />
              </div>
              <h2 className="text-xl font-bold text-white">Kết nối mã nguồn</h2>
            </div>
            <p className="text-[14px] text-[#94a3b8] mb-6">Chọn repository GitHub chứa mã nguồn ứng dụng của bạn</p>

            <div className="flex flex-col gap-3">
              {[
                { id: 'repo-1', name: 'my-saas-app', slug: 'user/my-saas-app' },
                { id: 'repo-2', name: 'landing-page', slug: 'user/landing-page' },
                { id: 'repo-3', name: 'api-service', slug: 'user/api-service' },
              ].map((repo) => (
                <div
                  key={repo.id}
                  className={cn(
                    "flex flex-col p-4 rounded-xl border cursor-pointer transition-all duration-200 hover:bg-[#131c31]",
                    selectedRepo === repo.id ? "border-[#0EA5E9] bg-[#0c1a2c]" : "border-[#1e293b] bg-[#0F172A]"
                  )}
                  onClick={() => setSelectedRepo(repo.id)}
                >
                  <span className="text-[15px] font-bold text-white leading-none">{repo.name}</span>
                  <span className="text-[13px] text-[#64748b] mt-1.5">{repo.slug}</span>
                </div>
              ))}
            </div>

            <div className="mt-8 flex justify-end">
              <button
                className={cn(
                  "px-6 py-2.5 rounded-lg text-[15px] font-semibold text-white shadow-sm transition-all w-full",
                  selectedRepo ? "bg-[#0EA5E9] hover:bg-[#0284c7]" : "bg-[#1e293b] text-[#64748b] cursor-not-allowed"
                )}
                disabled={!selectedRepo}
                onClick={() => setStep(2)}
              >
                Tiếp tục
              </button>
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div className="flex items-center gap-3 mb-2">
              <div className="text-[#38BDF8]">
                <Server className="size-6" />
              </div>
              <h2 className="text-xl font-bold text-white">Kết nối máy chủ</h2>
            </div>
            <p className="text-[14px] text-[#94a3b8] mb-6">Nhập thông tin SSH — hệ thống sẽ tự cài đặt agent</p>

            <div className="flex flex-col gap-5">
              <div className="flex flex-col gap-2">
                <label className="text-[14px] font-semibold text-white">Địa chỉ IP / Hostname</label>
                <input
                  type="text"
                  value={ip}
                  onChange={(e) => setIp(e.target.value)}
                  className="bg-[#0F172A] border border-[#0EA5E9] text-white rounded-lg px-4 py-2.5 text-[15px] focus:outline-none focus:ring-1 focus:ring-[#0EA5E9] shadow-[0_0_10px_rgba(14,165,233,0.2)]"
                  placeholder="Ví dụ: 192.168.1.100"
                />
              </div>

              <div className="flex flex-col gap-2">
                <label className="text-[14px] font-semibold text-white">Tên đăng nhập SSH</label>
                <input
                  type="text"
                  value={sshUser}
                  onChange={(e) => setSshUser(e.target.value)}
                  className="bg-[#0F172A] border border-[#1e293b] text-white rounded-lg px-4 py-2.5 text-[15px] focus:outline-none focus:border-[#0EA5E9]"
                  placeholder="Ví dụ: root"
                />
              </div>
            </div>

            <div className="mt-8 flex items-center justify-between gap-4">
              <button
                className="px-6 py-2.5 rounded-lg text-[15px] font-semibold text-white hover:bg-[#1e293b] transition-colors"
                onClick={() => setStep(1)}
              >
                Quay lại
              </button>
              <button
                className="flex-1 bg-[#0EA5E9] hover:bg-[#0284c7] px-6 py-2.5 rounded-lg text-[15px] font-semibold text-white shadow-sm transition-all text-center"
                onClick={() => setStep(3)}
              >
                Kết nối & cài agent
              </button>
            </div>
          </div>
        )}

        {step === 3 && (
          <div className="animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div className="flex items-center gap-3 mb-2">
              <div className="text-[#38BDF8]">
                <Layers className="size-6" />
              </div>
              <h2 className="text-xl font-bold text-white">Chọn dịch vụ đi kèm</h2>
            </div>
            <p className="text-[14px] text-[#94a3b8] mb-6">Bật các dịch vụ mà ứng dụng cần — hệ thống sẽ tự cài đặt và cấu hình trên VPS</p>

            <div className="flex flex-col gap-3">
              {[
                { id: 'postgresql', name: 'PostgreSQL', desc: 'Cơ sở dữ liệu quan hệ mạnh mẽ • Port 5432', icon: Database, color: "text-[#3b82f6]", bg: "bg-[#3b82f6]/10" },
                { id: 'mysql', name: 'MySQL', desc: 'Cơ sở dữ liệu phổ biến • Port 3306', icon: DatabaseZap, color: "text-[#f59e0b]", bg: "bg-[#f59e0b]/10" },
                { id: 'redis', name: 'Redis', desc: 'Bộ nhớ đệm key-value siêu nhanh • Port 6379', icon: Box, color: "text-[#ef4444]", bg: "bg-[#ef4444]/10" },
                { id: 'rabbitmq', name: 'RabbitMQ', desc: 'Hàng đợi tin nhắn (Message Queue) • Port 5672', icon: ServerCrash, color: "text-[#10b981]", bg: "bg-[#10b981]/10" },
                { id: 'kafka', name: 'Apache Kafka', desc: 'Nền tảng streaming sự kiện • Port 9092', icon: TerminalSquare, color: "text-[#8b5cf6]", bg: "bg-[#8b5cf6]/10" },
              ].map((svc) => {
                const Icon = svc.icon;
                const isChecked = services[svc.id as keyof typeof services];
                return (
                  <div key={svc.id} className="flex items-center justify-between p-4 rounded-xl border border-[#1e293b] bg-[#0F172A]">
                    <div className="flex items-center gap-4">
                      <div className="mt-1 flex-shrink-0">
                        <Icon className={cn("size-6", svc.color)} />
                      </div>
                      <div>
                        <div className="text-[15px] font-bold text-white">{svc.name}</div>
                        <div className="text-[13px] text-[#64748b] mt-0.5">{svc.desc}</div>
                      </div>
                    </div>
                    {/* Fake toggle switch */}
                    <div
                      className={cn(
                        "w-11 h-6 rounded-full flex items-center px-0.5 cursor-pointer transition-colors duration-200",
                        isChecked ? "bg-[#0EA5E9]" : "bg-[#1e293b]"
                      )}
                      onClick={() => setServices(prev => ({ ...prev, [svc.id]: !prev[svc.id as keyof typeof prev] }))}
                    >
                      <div
                        className={cn(
                          "w-5 h-5 bg-white rounded-full shadow-md transform transition-transform duration-200",
                          isChecked ? "translate-x-5" : ""
                        )}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
            
            <p className="text-[13px] text-[#64748b] mt-4 mb-2">Bạn có thể bỏ qua bước này nếu app không cần database hay message queue.</p>

            <div className="mt-4 flex items-center justify-between gap-4">
              <button
                className="px-6 py-2.5 rounded-lg text-[15px] font-semibold text-white hover:bg-[#1e293b] transition-colors"
                onClick={() => setStep(2)}
              >
                Quay lại
              </button>
              <button
                className="flex-1 bg-[#0EA5E9] hover:bg-[#0284c7] px-6 py-2.5 rounded-lg text-[15px] font-semibold text-white shadow-sm transition-all text-center"
                onClick={() => router.push('/deployments/new-demo')}
              >
                Tiếp tục
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
