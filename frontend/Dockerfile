# ── Build stage ──────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# ── Runtime stage ────────────────────────────────────────
FROM nginx:1.28-alpine

# Remove default config
RUN rm /etc/nginx/conf.d/default.conf

# Copy our template (envsubst fills in BACKEND_URL at startup)
COPY nginx.conf.template /etc/nginx/templates/default.conf.template

# Copy built SPA
COPY --from=builder /app/dist /usr/share/nginx/html

# Default backend URL (override at runtime via env var)
ENV BACKEND_URL=http://backend:8080

EXPOSE 80

