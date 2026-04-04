'use client';

import Link from 'next/link';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useRegister } from '@/lib/auth/auth-hooks';
import { registerSchema, type RegisterFormData } from '@/lib/schemas/auth-schemas';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';
import { getAuthErrorMessage } from '@/lib/auth/auth-errors';
import { AUTH_ERROR_CODES } from '@/lib/auth/auth-types';

export default function RegisterPage() {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegisterFormData>({
    resolver: zodResolver(registerSchema),
    defaultValues: { name: '', email: '', password: '' },
  });

  const registerMutation = useRegister();
  const serverError = registerMutation.error
    ? getAuthErrorMessage((registerMutation.error as { code?: string })?.code ?? AUTH_ERROR_CODES.UNKNOWN)
    : null;

  const onSubmit = (data: RegisterFormData) => {
    return registerMutation.mutateAsync(data);
  };

  return (
    <div className="auth-layout">
      <div className="auth-page">
        <h1>Create account</h1>
        <p className="mb-6 text-sm text-lazyops-muted">
          Get started with LazyOps
        </p>

        <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
          <FormField label="Name" error={errors.name?.message}>
            <FormInput
              type="text"
              autoComplete="name"
              placeholder="Your name"
              error={!!errors.name}
              {...register('name')}
            />
          </FormField>

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
              autoComplete="new-password"
              placeholder="Min 8 characters"
              error={!!errors.password}
              {...register('password')}
            />
          </FormField>

          {serverError && (
            <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
              {serverError}
            </div>
          )}

          <FormButton type="submit" loading={isSubmitting || registerMutation.isPending}>
            Create account
          </FormButton>
        </form>

        <p className="mt-6 text-center text-sm text-lazyops-muted">
          Already have an account?{' '}
          <Link href="/login" className="text-primary hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
