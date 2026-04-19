'use client';

import { useState } from 'react';
import { Modal } from '@/components/primitives/modal';
import { FormButton, FormField, FormInput } from '@/components/forms/form-fields';
import { useConnectProjectInfraSSH } from '@/modules/bootstrap/bootstrap-hooks';

type ProjectConnectInfraModalProps = {
  projectId: string;
  open: boolean;
  onClose: () => void;
  onSuccess?: () => void;
};

const defaultInfraForm = {
  instance_name: '',
  public_ip: '',
  ssh_host: '',
  ssh_port: '22',
  ssh_username: 'root',
  ssh_password: '',
  ssh_private_key: '',
  ssh_host_key_fingerprint: '',
};

export function ProjectConnectInfraModal({
  projectId,
  open,
  onClose,
  onSuccess,
}: ProjectConnectInfraModalProps) {
  const connectInfra = useConnectProjectInfraSSH(projectId);
  const [infraForm, setInfraForm] = useState(defaultInfraForm);
  const [showInfraAdvanced, setShowInfraAdvanced] = useState(false);
  const [infraFormError, setInfraFormError] = useState<string | null>(null);

  const resetState = () => {
    setInfraForm(defaultInfraForm);
    setShowInfraAdvanced(false);
    setInfraFormError(null);
  };

  const handleClose = () => {
    if (connectInfra.isPending) {
      return;
    }
    resetState();
    onClose();
  };

  const onConnectInfraSubmit = async () => {
    setInfraFormError(null);
    if (!infraForm.ssh_host.trim()) {
      setInfraFormError('Vui lòng nhập địa chỉ SSH host.');
      return;
    }
    if (!infraForm.ssh_username.trim()) {
      setInfraFormError('Vui lòng nhập SSH username.');
      return;
    }
    if (!infraForm.ssh_password.trim() && !infraForm.ssh_private_key.trim()) {
      setInfraFormError('Vui lòng nhập mật khẩu hoặc private key.');
      return;
    }

    try {
      await connectInfra.mutateAsync({
        instance_name: infraForm.instance_name.trim() || undefined,
        public_ip: infraForm.public_ip.trim() || undefined,
        ssh_host: infraForm.ssh_host.trim(),
        ssh_port: Number.parseInt(infraForm.ssh_port, 10) || 22,
        ssh_username: infraForm.ssh_username.trim(),
        ssh_password: infraForm.ssh_password || undefined,
        ssh_private_key: infraForm.ssh_private_key || undefined,
        ssh_host_key_fingerprint: infraForm.ssh_host_key_fingerprint.trim() || undefined,
        control_plane_url: typeof window !== 'undefined' ? window.location.origin : undefined,
      });
      resetState();
      onClose();
      onSuccess?.();
    } catch (err) {
      setInfraFormError(err instanceof Error ? err.message : 'Kết nối SSH thất bại');
    }
  };

  return (
    <Modal open={open} onClose={handleClose} title="Kết nối máy chủ qua SSH" size="lg">
      <div className="flex flex-col gap-4">
        <p className="text-sm text-lazyops-muted">
          Nhập thông tin SSH, LazyOps sẽ tự cài agent và tự gắn máy chủ vào project. Bạn không cần cấu hình cluster thủ công.
        </p>

        <FormField label="Tên máy chủ (tuỳ chọn)">
          <FormInput
            type="text"
            placeholder="prod-app-01"
            value={infraForm.instance_name}
            onChange={(event) => setInfraForm((prev) => ({ ...prev, instance_name: event.target.value }))}
          />
        </FormField>

        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="SSH host">
            <FormInput
              type="text"
              placeholder="203.0.113.10"
              value={infraForm.ssh_host}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_host: event.target.value }))}
            />
          </FormField>
          <FormField label="SSH port">
            <FormInput
              type="number"
              placeholder="22"
              value={infraForm.ssh_port}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_port: event.target.value }))}
            />
          </FormField>
        </div>

        <button
          type="button"
          className="w-fit rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-semibold text-lazyops-muted transition-colors hover:bg-lazyops-border/10"
          onClick={() => setShowInfraAdvanced((prev) => !prev)}
        >
          {showInfraAdvanced ? 'Ẩn cấu hình nâng cao' : 'Mở cấu hình nâng cao'}
        </button>

        {showInfraAdvanced ? (
          <FormField label="Public IP (tuỳ chọn)">
            <FormInput
              type="text"
              placeholder="203.0.113.10"
              value={infraForm.public_ip}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, public_ip: event.target.value }))}
            />
          </FormField>
        ) : null}

        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="SSH username">
            <FormInput
              type="text"
              placeholder="root"
              value={infraForm.ssh_username}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_username: event.target.value }))}
            />
          </FormField>
          <FormField label="Host key fingerprint (tuỳ chọn)">
            <FormInput
              type="text"
              placeholder="SHA256:..."
              value={infraForm.ssh_host_key_fingerprint}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_host_key_fingerprint: event.target.value }))}
            />
          </FormField>
        </div>

        <FormField label="Mật khẩu SSH (hoặc dùng private key)">
          <FormInput
            type="password"
            placeholder="••••••••"
            value={infraForm.ssh_password}
            onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_password: event.target.value }))}
          />
        </FormField>

        <FormField label="SSH private key (tuỳ chọn)">
          <textarea
            className="min-h-24 w-full rounded-lg border border-lazyops-border bg-lazyops-bg-accent/60 px-3 py-2 text-sm text-lazyops-text outline-none transition-colors placeholder:text-lazyops-muted/60 focus:border-primary/60 focus:ring-1 focus:ring-primary/30"
            placeholder="-----BEGIN OPENSSH PRIVATE KEY----- ..."
            value={infraForm.ssh_private_key}
            onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_private_key: event.target.value }))}
          />
        </FormField>

        {infraFormError ? (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {infraFormError}
          </div>
        ) : null}

        <FormButton
          type="button"
          loading={connectInfra.isPending}
          onClick={() => {
            void onConnectInfraSubmit();
          }}
        >
          Kết nối và cài agent
        </FormButton>
      </div>
    </Modal>
  );
}
