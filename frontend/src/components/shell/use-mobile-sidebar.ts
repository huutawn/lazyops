'use client';

import { useState, useCallback } from 'react';

export function useMobileSidebar() {
  const [open, setOpen] = useState(false);

  const toggle = useCallback(() => setOpen((prev) => !prev), []);
  const close = useCallback(() => setOpen(false), []);

  return { open, toggle, close };
}
