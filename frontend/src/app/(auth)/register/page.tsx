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
    <div className="flex flex-col gap-8 animate-in fade-in slide-in-from-bottom-6 duration-500">
      <div className="space-y-2 text-center lg:text-left">
        <h1 className="text-3xl font-semibold tracking-tight text-foreground">Create account</h1>
        <p className="text-sm text-muted-foreground">Get started with LazyOps</p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-5" noValidate>
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
          <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm font-medium text-destructive animate-in fade-in zoom-in-95">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || registerMutation.isPending} className="mt-2 h-11">
          Create account
        </FormButton>
      </form>

      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{' '}
        <Link href="/login" className="font-semibold text-primary transition-colors hover:text-primary/80 hover:underline">
          Sign in
        </Link>
      </p>
    </div>
  );
}
