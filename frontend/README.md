# LazyOps Frontend

Day 3 scaffold for the LazyOps operator console.

## Stack

- Next.js 16 App Router
- TypeScript strict mode
- ESLint
- Prettier
- Vitest

## Requirements

- Node.js 20+
- npm 10+

## Local setup

```bash
cp .env.example .env.local
npm install
npm run dev
```

Open `http://localhost:3000`.

## Available scripts

```bash
npm run dev
npm run build
npm run lint
npm run typecheck
npm run test
```

## Day 3 scope

This scaffold intentionally includes only:

- standalone frontend app bootstrap
- baseline package/config files
- `src/app` shell with a starter landing page
- path alias support
- lint, typecheck, test, and build commands

It does not yet include the Day 4 module architecture or feature implementation.
