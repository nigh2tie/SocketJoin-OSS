# Build Stage
FROM node:24.14.0-alpine3.23 AS builder

WORKDIR /app

# Copy package files and install dependencies
COPY package*.json ./
RUN npm ci

# Copy source code and build the SPA
COPY . .
RUN npm run build

# Run Stage
FROM nginx:1.29.7-alpine3.23

# Copy the compiled static assets
COPY --from=builder /app/build /usr/share/nginx/html

# Copy the Nginx SPA configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
