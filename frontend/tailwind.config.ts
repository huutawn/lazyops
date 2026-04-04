/** @type {import('tailwindcss').Config} */
const tailwindConfig = {
  content: [
    './src/**/*.{ts,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        border: 'var(--border)',
        input: 'var(--input)',
        ring: 'var(--ring)',
        background: 'var(--background)',
        foreground: 'var(--foreground)',
        primary: {
          DEFAULT: 'var(--primary)',
          foreground: 'var(--primary-foreground)',
        },
        secondary: {
          DEFAULT: 'var(--secondary)',
          foreground: 'var(--secondary-foreground)',
        },
        destructive: {
          DEFAULT: 'var(--destructive)',
          foreground: 'var(--destructive-foreground)',
        },
        muted: {
          DEFAULT: 'var(--muted)',
          foreground: 'var(--muted-foreground)',
        },
        accent: {
          DEFAULT: 'var(--accent)',
          foreground: 'var(--accent-foreground)',
        },
        popover: {
          DEFAULT: 'var(--popover)',
          foreground: 'var(--popover-foreground)',
        },
        card: {
          DEFAULT: 'var(--card)',
          foreground: 'var(--card-foreground)',
        },
        success: {
          DEFAULT: 'var(--success)',
          foreground: 'var(--success-foreground)',
        },
        warning: {
          DEFAULT: 'var(--warning)',
          foreground: 'var(--warning-foreground)',
        },
        info: {
          DEFAULT: 'var(--info)',
          foreground: 'var(--info-foreground)',
        },
        lazyops: {
          bg: 'var(--lazyops-bg)',
          'bg-accent': 'var(--lazyops-bg-accent)',
          card: 'var(--lazyops-card)',
          border: 'var(--lazyops-border)',
          text: 'var(--lazyops-text)',
          muted: 'var(--lazyops-muted)',
          shadow: 'var(--lazyops-shadow)',
        },
        health: {
          healthy: 'var(--health-healthy)',
          degraded: 'var(--health-degraded)',
          unhealthy: 'var(--health-unhealthy)',
          offline: 'var(--health-offline)',
          unknown: 'var(--health-unknown)',
        },
        rollout: {
          progress: 'var(--rollout-progress)',
          paused: 'var(--rollout-paused)',
          failed: 'var(--rollout-failed)',
          completed: 'var(--rollout-completed)',
        },
        incident: {
          critical: 'var(--incident-critical)',
          warning: 'var(--incident-warning)',
          resolved: 'var(--incident-resolved)',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
};

export default tailwindConfig;
