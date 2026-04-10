# syntax=ghcr.io/docker/dockerfile:1.7

FROM node:22-alpine AS deps

WORKDIR /app

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

FROM node:22-alpine AS builder

WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ARG INTERNAL_API_URL=http://backend:8080
ARG NEXT_PUBLIC_AGENT_IMAGE_DEFAULT=tawn/lazyops-agent:latest

ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV INTERNAL_API_URL=${INTERNAL_API_URL}
ENV NEXT_PUBLIC_AGENT_IMAGE_DEFAULT=${NEXT_PUBLIC_AGENT_IMAGE_DEFAULT}

COPY --from=deps /app/node_modules ./node_modules
COPY . .

RUN npm run build

FROM node:22-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

RUN addgroup -S nodejs \
    && adduser -S nextjs -G nodejs

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:3000 >/dev/null || exit 1

CMD ["node", "server.js"]
