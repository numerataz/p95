# Stage 1: Build
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files (from web/ directory since context is repo root)
COPY web/package*.json ./

# Install dependencies
RUN npm ci

# Copy source
COPY web/ .

# Build args for environment
ARG VITE_API_URL=/api/v1
ENV VITE_API_URL=$VITE_API_URL

# Build static files
RUN npm run build

# Stage 2: Serve with nginx
FROM nginx:alpine

# Copy custom nginx config
COPY web/nginx.conf /etc/nginx/conf.d/default.conf

# Copy built files
COPY --from=builder /app/dist /usr/share/nginx/html

# Expose port
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
