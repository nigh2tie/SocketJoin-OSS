# Build Stage
FROM node:22-alpine AS builder

WORKDIR /app

# Copy package files and install dependencies
COPY package*.json ./
RUN npm install

# Copy source code and build the SPA
COPY . .
RUN npm run build

# Run Stage
FROM nginx:alpine

# Copy the compiled static assets
COPY --from=builder /app/build /usr/share/nginx/html

# Copy the Nginx SPA configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
