import type { ProjectBootstrapStatus, BootstrapStep } from '@/modules/bootstrap/bootstrap-types';
import type { ProjectRepoLink } from '@/modules/repo-link/repo-link-types';

export type ProjectSetupCard = {
  id: 'connect_code' | 'connect_infra' | 'deploy';
  title: string;
  state: string;
  summary: string;
};

export type ProjectNextAction =
  | {
      kind: 'repo';
      label: string;
      description: string;
    }
  | {
      kind: 'infra';
      label: string;
      description: string;
    }
  | {
      kind: 'deploy';
      label: string;
      description: string;
    }
  | {
      kind: 'open';
      label: string;
      description: string;
      href: string;
    }
  | {
      kind: 'logs';
      label: string;
      description: string;
      href: string;
    };

const STEP_TITLES: Record<ProjectSetupCard['id'], string> = {
  connect_code: 'Mã nguồn',
  connect_infra: 'Máy chủ',
  deploy: 'Triển khai',
};

export function formatBootstrapStateLabelVN(value: string): string {
  const normalized = value.toLowerCase();
  const map: Record<string, string> = {
    missing: 'Chưa kết nối',
    linked: 'Đã liên kết',
    healthy: 'Sẵn sàng',
    installing: 'Đang cài',
    ready: 'Sẵn sàng',
    blocked: 'Bị chặn',
    deploying: 'Đang triển khai',
    degraded: 'Cần xử lý',
    rolled_back: 'Đã hoàn tác',
    error: 'Lỗi',
    running: 'Đang chạy',
    attention_required: 'Cần xử lý',
    ready_to_deploy: 'Sẵn sàng triển khai',
    partially_ready: 'Chưa hoàn tất',
    not_ready: 'Chưa sẵn sàng',
    completed: 'Hoàn tất',
    success: 'Thành công',
    pending: 'Chờ xử lý',
    failed: 'Thất bại',
    started: 'Đã bắt đầu',
    queued: 'Đang xếp hàng',
    promoted: 'Đã phát hành',
  };

  if (map[normalized]) {
    return map[normalized];
  }

  return value.replace(/_/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase());
}

export function summarizeProjectSetup(status: ProjectBootstrapStatus): ProjectSetupCard[] {
  const code = findBootstrapStep(status, 'connect_code');
  const infra = findBootstrapStep(status, 'connect_infra');
  const deploy = findBootstrapStep(status, 'deploy');

  return [
    {
      id: 'connect_code',
      title: STEP_TITLES.connect_code,
      state: code?.state ?? 'missing',
      summary: code?.summary ?? 'Chưa kết nối repository',
    },
    {
      id: 'connect_infra',
      title: STEP_TITLES.connect_infra,
      state: infra?.state ?? 'missing',
      summary: infra?.summary ?? 'Chưa kết nối máy chủ',
    },
    {
      id: 'deploy',
      title: STEP_TITLES.deploy,
      state: deploy?.state ?? 'blocked',
      summary: deploy?.summary ?? 'Chưa thể triển khai',
    },
  ];
}

export function resolveProjectNextAction({
  status,
  repoLink,
  primaryPublicURL,
  logsHref,
}: {
  status: ProjectBootstrapStatus;
  repoLink: ProjectRepoLink | null;
  primaryPublicURL?: string;
  logsHref: string;
}): ProjectNextAction {
  const code = findBootstrapStep(status, 'connect_code');
  const infra = findBootstrapStep(status, 'connect_infra');
  const deploy = findBootstrapStep(status, 'deploy');

  if (primaryPublicURL && isServingState(deploy?.state)) {
    return {
      kind: 'open',
      label: 'Mở website',
      description: 'Project đã có địa chỉ public. Bạn có thể mở ngay để kiểm tra bản đang chạy.',
      href: primaryPublicURL,
    };
  }

  if (!repoLink || isMissingState(code?.state)) {
    return {
      kind: 'repo',
      label: 'Kết nối mã nguồn',
      description: code?.summary || 'Hãy chọn repository và nhánh theo dõi trước khi deploy.',
    };
  }

  if (isMissingState(infra?.state)) {
    return {
      kind: 'infra',
      label: 'Kết nối máy chủ',
      description: infra?.summary || 'Project đã biết source code, bước tiếp theo là nối VPS để chạy.',
    };
  }

  if (deploy?.state === 'deploying') {
    return {
      kind: 'logs',
      label: 'Xem nhật ký',
      description: deploy.summary || 'Bản triển khai đang chạy. Mở nhật ký để theo dõi tiến trình.',
      href: logsHref,
    };
  }

  if (!isServingState(deploy?.state)) {
    return {
      kind: 'deploy',
      label: 'Triển khai ngay',
      description: deploy?.summary || 'Mọi thứ đã sẵn sàng. Bạn có thể chạy deployment đầu tiên ngay bây giờ.',
    };
  }

  return {
    kind: 'logs',
    label: 'Xem nhật ký',
    description: 'Project đã sẵn sàng. Mở nhật ký và runtime để theo dõi trạng thái hiện tại.',
    href: logsHref,
  };
}

function findBootstrapStep(status: ProjectBootstrapStatus, id: ProjectSetupCard['id']): BootstrapStep | undefined {
  return status.steps.find((step) => step.id === id);
}

function isMissingState(state?: string) {
  return !state || state === 'missing' || state === 'blocked' || state === 'error';
}

function isServingState(state?: string) {
  return state === 'healthy' || state === 'running' || state === 'promoted';
}
