'use client';

import { useEffect, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import {
  useCreateInstance,
  useInstallInstanceAgentViaSSH,
  useInstances,
  useIssueInstanceBootstrapToken,
} from '@/modules/instances/instance-hooks';
import {
  createInstanceSchema,
  type BootstrapToken,
  type CreateInstanceFormData,
  type CreateInstanceResponse,
  type InstallInstanceAgentSSHRequest,
} from '@/modules/instances/instance-types';
import { getStatusVariant, formatStatus } from '@/modules/instances/instance-utils';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

function normalizeControlPlaneURL(value: string): string {
  return value.replace(/\/api\/v1\/?$/, '').replace(/\/+$/, '');
}

function shouldAutoReplaceControlPlaneURL(value: string): boolean {
  const lower = value.trim().toLowerCase();
  if (!lower) {
    return true;
  }
  return (
    lower.includes("localhost") ||
    lower.includes("127.0.0.1") ||
    lower.includes("0.0.0.0")
  );
}

export default function InstancesPage() {
  const { data, isLoading, isError } = useInstances();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showBootstrapModal, setShowBootstrapModal] = useState(false);
  const [bootstrapTokensByInstanceID, setBootstrapTokensByInstanceID] = useState<Record<string, BootstrapToken>>({});
  const [bootstrapToken, setBootstrapToken] = useState<BootstrapToken | null>(null);
  const [bootstrapInstanceID, setBootstrapInstanceID] = useState<string | null>(null);
  const issueBootstrapToken = useIssueInstanceBootstrapToken();
  const installViaSSH = useInstallInstanceAgentViaSSH();

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Instances" subtitle="Manage your target machines" />
        <ErrorState title="Failed to load instances" message="Could not fetch instance data. Please try again." />
      </div>
    );
  }

  const instances = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Instances"
        subtitle="Register machines for LazyOps to manage and deploy onto."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowCreateModal(true)}
          >
            Add instance
          </button>
        }
      />

      {instances.length === 0 ? (
        <SectionCard title="No instances" description="Add your first machine to get started.">
          <EmptyState
            title="No instances registered"
            description="Create an instance to register a target machine. You'll get a bootstrap command to install the LazyOps agent."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowCreateModal(true)}
              >
                Add instance
              </button>
            }
          />
        </SectionCard>
      ) : (
        <SectionCard>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Name</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Public IP</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Private IP</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Labels</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Agent</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Actions</th>
                </tr>
              </thead>
              <tbody>
                {instances.map((instance) => (
                  <tr
                    key={instance.id}
                    className="border-b border-lazyops-border/50 transition-colors hover:bg-lazyops-border/10"
                  >
                    <td className="px-4 py-3 font-medium text-lazyops-text">{instance.name}</td>
                    <td className="px-4 py-3">
                      <StatusBadge
                        label={formatStatus(instance.status)}
                        variant={getStatusVariant(instance.status)}
                        size="sm"
                      />
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-lazyops-muted">
                      {instance.public_ip ?? '—'}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-lazyops-muted">
                      {instance.private_ip ?? '—'}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {Object.entries(instance.labels).map(([key, value]) => (
                          <span
                            key={key}
                            className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-[10px] text-lazyops-muted"
                          >
                            {key}{value ? `:${value}` : ''}
                          </span>
                        ))}
                        {Object.keys(instance.labels).length === 0 && (
                          <span className="text-xs text-lazyops-muted/50">—</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs text-lazyops-muted">
                      {instance.agent_id ? (
                        <span className="font-mono">{instance.agent_id.slice(0, 12)}…</span>
                      ) : (
                        <span className="text-lazyops-muted/50">Not enrolled</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {instance.status !== 'online' && (
                        <>
                          {bootstrapTokensByInstanceID[instance.id] ? (
                            <button
                              type="button"
                              className="text-xs text-primary hover:underline"
                              onClick={() => {
                                setBootstrapInstanceID(instance.id);
                                setBootstrapToken(bootstrapTokensByInstanceID[instance.id]);
                                setShowBootstrapModal(true);
                              }}
                            >
                              View bootstrap
                            </button>
                          ) : (
                            <button
                              type="button"
                              className="text-xs text-lazyops-muted/60 hover:text-lazyops-text hover:underline"
                              onClick={() => {
                                setBootstrapInstanceID(instance.id);
                                setBootstrapToken(null);
                                setShowBootstrapModal(true);
                              }}
                            >
                              Bootstrap help
                            </button>
                          )}
                        </>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </SectionCard>
      )}

      <CreateInstanceModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSuccess={(result) => {
          setBootstrapTokensByInstanceID((prev) => ({
            ...prev,
            [result.instance.id]: result.bootstrap,
          }));
          setBootstrapInstanceID(result.instance.id);
          setBootstrapToken(result.bootstrap);
          setShowCreateModal(false);
          setShowBootstrapModal(true);
        }}
      />

      <BootstrapModal
        open={showBootstrapModal}
        onClose={() => {
          setShowBootstrapModal(false);
          setBootstrapInstanceID(null);
          setBootstrapToken(null);
          issueBootstrapToken.reset();
          installViaSSH.reset();
        }}
        instanceID={bootstrapInstanceID}
        onRegenerate={() => {
          if (!bootstrapInstanceID) {
            return Promise.resolve();
          }

          return issueBootstrapToken.mutateAsync(bootstrapInstanceID).then((issuedToken) => {
            setBootstrapTokensByInstanceID((prev) => ({
              ...prev,
              [bootstrapInstanceID]: issuedToken,
            }));
            setBootstrapToken(issuedToken);
          });
        }}
        regenerateLoading={issueBootstrapToken.isPending}
        regenerateError={issueBootstrapToken.error?.message ?? null}
        onInstallViaSSH={(data) => {
          if (!bootstrapInstanceID) {
            return Promise.resolve();
          }

          return installViaSSH.mutateAsync({ instanceID: bootstrapInstanceID, data }).then((result) => {
            setBootstrapTokensByInstanceID((prev) => ({
              ...prev,
              [bootstrapInstanceID]: result.bootstrap,
            }));
            setBootstrapToken(result.bootstrap);
          });
        }}
        installLoading={installViaSSH.isPending}
        installError={installViaSSH.error?.message ?? null}
        installHostFingerprint={installViaSSH.data?.host_key_fingerprint ?? null}
        token={bootstrapToken}
      />
    </div>
  );
}

type CreateInstanceModalProps = {
  open: boolean;
  onClose: () => void;
  onSuccess: (result: CreateInstanceResponse) => void;
};

function CreateInstanceModal({ open, onClose, onSuccess }: CreateInstanceModalProps) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<CreateInstanceFormData>({
    resolver: zodResolver(createInstanceSchema),
    defaultValues: { name: '', public_ip: '', private_ip: '', labels: '' },
  });

  const createInstance = useCreateInstance();
  const serverError = createInstance.error?.message ?? null;

  const onSubmit = (data: CreateInstanceFormData) => {
    return createInstance.mutateAsync(data).then((result) => {
      onSuccess(result);
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Add instance" size="md">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <FormField label="Instance name" error={errors.name?.message}>
          <FormInput
            type="text"
            placeholder="prod-web-01"
            error={!!errors.name}
            {...register('name')}
          />
        </FormField>

        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="Public IP" error={errors.public_ip?.message}>
            <FormInput
              type="text"
              placeholder="203.0.113.10"
              error={!!errors.public_ip}
              {...register('public_ip')}
            />
          </FormField>

          <FormField label="Private IP" error={errors.private_ip?.message}>
            <FormInput
              type="text"
              placeholder="10.0.1.10"
              error={!!errors.private_ip}
              {...register('private_ip')}
            />
          </FormField>
        </div>

        <FormField label="Labels" error={errors.labels?.message}>
          <FormInput
            type="text"
            placeholder="env:prod, role:web"
            error={!!errors.labels}
            {...register('labels')}
          />
          <p className="mt-1 text-[10px] text-lazyops-muted/60">
            Comma-separated key:value pairs. Keys are lowercased automatically.
          </p>
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || createInstance.isPending}>
          Create instance
        </FormButton>
      </form>
    </Modal>
  );
}

type BootstrapModalProps = {
  open: boolean;
  onClose: () => void;
  instanceID: string | null;
  onRegenerate: () => Promise<void>;
  regenerateLoading: boolean;
  regenerateError: string | null;
  onInstallViaSSH: (data: InstallInstanceAgentSSHRequest) => Promise<void>;
  installLoading: boolean;
  installError: string | null;
  installHostFingerprint: string | null;
  token: BootstrapToken | null;
};

function BootstrapModal({
  open,
  onClose,
  instanceID,
  onRegenerate,
  regenerateLoading,
  regenerateError,
  onInstallViaSSH,
  installLoading,
  installError,
  installHostFingerprint,
  token,
}: BootstrapModalProps) {
  const [copied, setCopied] = useState(false);
  const [sshHost, setSSHHost] = useState('');
  const [sshPort, setSSHPort] = useState('22');
  const [sshUser, setSSHUser] = useState('root');
  const [sshPassword, setSSHPassword] = useState('');
  const [sshPrivateKey, setSSHPrivateKey] = useState('');
  const [sshHostKeyFingerprint, setSSHHostKeyFingerprint] = useState('');
  const [controlPlaneURL, setControlPlaneURL] = useState(() =>
    normalizeControlPlaneURL(process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'),
  );
  const [agentImage, setAgentImage] = useState(
    process.env.NEXT_PUBLIC_AGENT_IMAGE_DEFAULT ?? 'tawn/lazyops-agent:latest',
  );

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    if (!shouldAutoReplaceControlPlaneURL(controlPlaneURL)) {
      return;
    }
    setControlPlaneURL(normalizeControlPlaneURL(window.location.origin));
  }, [controlPlaneURL]);

  const resolvedControlPlaneURL =
    controlPlaneURL.trim() ||
    (typeof window !== 'undefined' ? normalizeControlPlaneURL(window.location.origin) : '');

  const command = token
    ? `AGENT_BOOTSTRAP_TOKEN='${token.token}' AGENT_STATE_ENCRYPTION_KEY="$(openssl rand -hex 32)" AGENT_CONTROL_PLANE_URL='${resolvedControlPlaneURL || 'http://<your-backend-host>'}' go run ./cmd/server`
    : 'Click "Regenerate token" to issue a fresh bootstrap token for this instance.';

  const handleCopy = async () => {
    await navigator.clipboard.writeText(command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Modal open={open} onClose={onClose} title="Bootstrap command" size="lg">
      <div className="flex flex-col gap-4">
        {token ? (
          <p className="text-sm text-lazyops-muted">
            Run this command from the <code>agent</code> directory on your target machine to enroll the LazyOps
            agent.
          </p>
        ) : (
          <p className="text-sm text-lazyops-muted">
            No reusable bootstrap token is available for this instance in the current session.
          </p>
        )}

        <div className="relative rounded-lg border border-lazyops-border bg-lazyops-bg p-4">
          <code className="block break-all text-sm text-lazyops-text">{command}</code>
          <button
            type="button"
            className="absolute right-3 top-3 rounded-md border border-lazyops-border bg-lazyops-bg-accent px-2.5 py-1.5 text-xs text-lazyops-muted transition-colors hover:text-lazyops-text"
            onClick={handleCopy}
          >
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>

        {instanceID ? (
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="rounded-md border border-lazyops-border bg-lazyops-bg-accent px-3 py-1.5 text-xs text-lazyops-text transition-colors hover:bg-lazyops-border/20 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={() => {
                void onRegenerate();
              }}
              disabled={regenerateLoading}
            >
              {regenerateLoading ? 'Regenerating…' : 'Regenerate token'}
            </button>
          </div>
        ) : null}

        {regenerateError ? (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {regenerateError}
          </div>
        ) : null}

        {instanceID ? (
          <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/40 p-3">
            <p className="mb-3 text-xs font-medium text-lazyops-text">Install Agent Via SSH</p>
            <div className="grid gap-3 sm:grid-cols-2">
              <FormInput
                type="text"
                placeholder="SSH host (e.g. 203.0.113.10)"
                value={sshHost}
                onChange={(event) => setSSHHost(event.target.value)}
              />
              <FormInput
                type="number"
                placeholder="SSH port"
                value={sshPort}
                onChange={(event) => setSSHPort(event.target.value)}
              />
              <FormInput
                type="text"
                placeholder="SSH username"
                value={sshUser}
                onChange={(event) => setSSHUser(event.target.value)}
              />
              <FormInput
                type="password"
                placeholder="SSH password (optional if key is used)"
                value={sshPassword}
                onChange={(event) => setSSHPassword(event.target.value)}
              />
            </div>
            <textarea
              className="mt-3 min-h-24 w-full rounded-lg border border-lazyops-border bg-lazyops-bg px-3 py-2 text-xs text-lazyops-text outline-none transition-colors placeholder:text-lazyops-muted/60 focus:border-primary/60 focus:ring-1 focus:ring-primary/30"
              placeholder="SSH private key (optional)"
              value={sshPrivateKey}
              onChange={(event) => setSSHPrivateKey(event.target.value)}
            />
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              <FormInput
                type="text"
                placeholder="Host key fingerprint (optional, SHA256:...)"
                value={sshHostKeyFingerprint}
                onChange={(event) => setSSHHostKeyFingerprint(event.target.value)}
              />
              <FormInput
                type="text"
                placeholder="Control plane URL"
                value={controlPlaneURL}
                onChange={(event) => setControlPlaneURL(event.target.value)}
              />
              <FormInput
                type="text"
                placeholder="Agent image"
                value={agentImage}
                onChange={(event) => setAgentImage(event.target.value)}
                className="sm:col-span-2"
              />
            </div>
            <div className="mt-3 flex items-center gap-2">
              <button
                type="button"
                className="rounded-md border border-lazyops-border bg-lazyops-bg px-3 py-1.5 text-xs text-lazyops-text transition-colors hover:bg-lazyops-border/20 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => {
                  const portValue = Number.parseInt(sshPort, 10);
                  void onInstallViaSSH({
                    host: sshHost.trim(),
                    port: Number.isFinite(portValue) ? portValue : 22,
                    username: sshUser.trim(),
                    password: sshPassword,
                    private_key: sshPrivateKey,
                    host_key_fingerprint: sshHostKeyFingerprint.trim() || undefined,
                    control_plane_url: resolvedControlPlaneURL,
                    runtime_mode: 'standalone',
                    agent_kind: 'instance_agent',
                    agent_image: agentImage.trim() || undefined,
                  });
                }}
                disabled={installLoading}
              >
                {installLoading ? 'Installing…' : 'Install via SSH'}
              </button>
            </div>

            {installHostFingerprint ? (
              <p className="mt-2 text-[11px] text-lazyops-muted">
                Remote host key: <span className="font-mono">{installHostFingerprint}</span>
              </p>
            ) : null}

            {installError ? (
              <div className="mt-2 rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
                {installError}
              </div>
            ) : null}
          </div>
        ) : null}

        {token ? (
          <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/50 px-3 py-2 text-xs text-lazyops-muted">
            Token expires at: {new Date(token.expires_at).toLocaleString()}
          </div>
        ) : null}

        <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/50 px-3 py-3 text-xs text-lazyops-muted">
          <p className="mb-1 font-medium text-lazyops-text">What happens next?</p>
          <ol className="list-decimal space-y-1 pl-4">
            <li>The agent installs on the target machine</li>
            <li>It enrolls using the bootstrap token</li>
            <li>Instance status changes to <span className="text-health-healthy">online</span></li>
            <li>LazyOps can now deploy services to this target</li>
          </ol>
        </div>
      </div>
    </Modal>
  );
}
