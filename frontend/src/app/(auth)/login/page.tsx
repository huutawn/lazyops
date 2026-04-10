'use client';

import { Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useLogin } from '@/lib/auth/auth-hooks';
import { loginSchema, type LoginFormData } from '@/lib/schemas/auth-schemas';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';
import { getAuthErrorMessage } from '@/lib/auth/auth-errors';
import { AUTH_ERROR_CODES } from '@/lib/auth/auth-types';

const OAUTH_ERROR_MESSAGES: Record<string, string> = {
  oauth_denied: 'Bạn đã huỷ hoặc từ chối đăng nhập.',
  oauth_missing_params: 'Thiếu tham số bắt buộc trong callback OAuth.',
  oauth_network: 'Lỗi mạng trong quá trình đăng nhập OAuth. Vui lòng thử lại.',
  oauth_failed: 'Đăng nhập OAuth thất bại. Vui lòng thử lại.',
};

function LoginForm() {
  const searchParams = useSearchParams();
  const oauthError = searchParams.get('error');
  const oauthErrorDesc = searchParams.get('error_description');
  const redirect = searchParams.get('redirect');

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: '', password: '' },
  });

  const login = useLogin();
  const serverError = login.error
    ? getAuthErrorMessage((login.error as { code?: string })?.code ?? AUTH_ERROR_CODES.UNKNOWN)
    : null;

  const oauthMessage = oauthError
    ? (OAUTH_ERROR_MESSAGES[oauthError] ?? oauthErrorDesc ?? 'OAuth error occurred.')
    : null;
  const displayError = serverError ?? oauthMessage;

  const onSubmit = (data: LoginFormData) => {
    return login.mutateAsync({ ...data, redirect: redirect ?? undefined });
  };

  return (
    <div className="flex flex-col gap-8 animate-in fade-in slide-in-from-bottom-6 duration-500">
      <div className="space-y-2 text-center lg:text-left">
        <h1 className="text-3xl font-semibold tracking-tight text-foreground">Đăng nhập</h1>
        <p className="text-sm text-muted-foreground">Truy cập bảng điều khiển LazyOps</p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-5" noValidate>
        <FormField label="Email" error={errors.email?.message}>
          <FormInput
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            error={!!errors.email}
            {...register('email')}
          />
        </FormField>

        <FormField label="Mật khẩu" error={errors.password?.message}>
          <FormInput
            type="password"
            autoComplete="current-password"
            placeholder="••••••••"
            error={!!errors.password}
            {...register('password')}
          />
        </FormField>

        {displayError && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm font-medium text-destructive animate-in fade-in zoom-in-95">
            {displayError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || login.isPending} className="mt-2 h-11">
          Vào bảng điều khiển
        </FormButton>
      </form>

      <div className="flex flex-col gap-4">
        <div className="relative">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-border" />
          </div>
          <div className="relative flex justify-center text-xs uppercase">
            <span className="bg-background px-2 text-muted-foreground">Hoặc đăng nhập bằng</span>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <a
            href="/api/auth/oauth/google/start"
            className="flex h-11 items-center justify-center gap-2 rounded-lg border border-border bg-card/50 text-sm font-medium text-foreground transition-all hover:bg-card hover:border-primary/50 hover:shadow-sm"
          >
            Google
          </a>
          <a
            href="/api/auth/oauth/github/start"
            className="flex h-11 items-center justify-center gap-2 rounded-lg border border-border bg-card/50 text-sm font-medium text-foreground transition-all hover:bg-card hover:border-primary/50 hover:shadow-sm"
          >
            GitHub
          </a>
        </div>
      </div>

      <p className="text-center text-sm text-muted-foreground">
        Chưa có tài khoản?{' '}
        <Link href="/register" className="font-semibold text-primary transition-colors hover:text-primary/80 hover:underline">
          Tạo tài khoản
        </Link>
      </p>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}
