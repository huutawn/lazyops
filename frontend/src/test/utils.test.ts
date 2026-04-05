import { describe, expect, it } from 'vitest';
import { cn } from '@/lib/utils';

describe('cn utility', () => {
  it('merges class strings', () => {
    expect(cn('foo', 'bar')).toBe('foo bar');
  });

  it('merges tailwind classes with twMerge', () => {
    expect(cn('px-2', 'px-4')).toBe('px-4');
  });

  it('filters out falsy values', () => {
    expect(cn('foo', false, undefined, null, 'bar')).toBe('foo bar');
  });

  it('handles conditional classes', () => {
    expect(cn('base', true && 'active', false && 'inactive')).toBe('base active');
  });

  it('handles empty input', () => {
    expect(cn()).toBe('');
  });
});
