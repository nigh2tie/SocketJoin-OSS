# Stage 1: Frontend build
FROM node:22-alpine AS frontend
WORKDIR /view
COPY view/package*.json ./
RUN npm ci
COPY view/ ./
RUN npm run build

# Stage 2: Go build
FROM golang:1.24.5-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o socketjoin-api ./cmd/api/main.go
CMD ["/app/socketjoin-api"]

# Stage 3: Runtime (includes frontend assets for Cloud Run single-container deploy)
FROM alpine:latest
WORKDIR /app
COPY --from=backend /app/socketjoin-api .
COPY --from=backend /app/migrations ./migrations
COPY --from=frontend /view/build ./view/build
EXPOSE 8080
CMD ["./socketjoin-api"]
