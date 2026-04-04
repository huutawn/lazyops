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
  oauth_denied: 'Authentication was cancelled or denied.',
  oauth_missing_params: 'OAuth callback missing required parameters.',
  oauth_network: 'Network error during OAuth flow. Please try again.',
  oauth_failed: 'OAuth authentication failed. Please try again.',
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
    <div className="auth-layout">
      <div className="auth-page">
        <h1>Sign in</h1>
        <p className="mb-6 text-sm text-lazyops-muted">Access your LazyOps console</p>

        <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
          <FormField label="Email" error={errors.email?.message}>
            <FormInput
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              error={!!errors.email}
              {...register('email')}
            />
          </FormField>

          <FormField label="Password" error={errors.password?.message}>
            <FormInput
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              error={!!errors.password}
              {...register('password')}
            />
          </FormField>

          {displayError && (
            <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
              {displayError}
            </div>
          )}

          <FormButton type="submit" loading={isSubmitting || login.isPending}>
            Sign in
          </FormButton>
        </form>

        <div className="mt-6 flex flex-col gap-3">
          <div className="flex items-center gap-3 before:h-px before:flex-1 before:bg-lazyops-border after:h-px after:flex-1 after:bg-lazyops-border">
            <span className="text-xs text-lazyops-muted">or</span>
          </div>

          <a
            href="/api/auth/oauth/google/start"
            className="flex h-10 items-center justify-center rounded-lg border border-lazyops-border bg-lazyops-bg-accent/60 text-sm text-lazyops-text transition-colors hover:bg-lazyops-bg-accent"
          >
            Continue with Google
          </a>

          <a
            href="/api/auth/oauth/github/start"
            className="flex h-10 items-center justify-center rounded-lg border border-lazyops-border bg-lazyops-bg-accent/60 text-sm text-lazyops-text transition-colors hover:bg-lazyops-bg-accent"
          >
            Continue with GitHub
          </a>
        </div>

        <p className="mt-6 text-center text-sm text-lazyops-muted">
          Don&apos;t have an account?{' '}
          <Link href="/register" className="text-primary hover:underline">
            Create one
          </Link>
        </p>
      </div>
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
