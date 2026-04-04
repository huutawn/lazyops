import { NextResponse } from 'next/server';
import { API_BASE_URL } from '@/lib/auth/auth-config';

export async function GET() {
  return NextResponse.redirect(`${API_BASE_URL}/api/v1/auth/oauth/google/start`);
}
