# Multi-stage build for Piano PIR Wallet
# Stage 1: Build React app with Vite
FROM node:18-alpine AS builder

WORKDIR /app

# Copy package files
COPY package.json ./

# BuildKit cache mount for npm cache
RUN --mount=type=cache,target=/root/.npm \
    npm install --prefer-offline --no-audit

# Copy source code
COPY . .

# Build for production
RUN npm run build

# Stage 2: Serve with nginx
FROM nginx:alpine

# Copy built assets from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose HTTP port
EXPOSE 5173

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
