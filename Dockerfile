# Stage 1: builder
FROM golang:1.24.6-alpine AS builder
RUN apk --no-cache add ca-certificates git
WORKDIR /app

# deps
COPY go.mod go.sum ./
RUN go mod download

# src
COPY . .

# build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/fetcher ./cmd/fetcher

# Stage 2: runner
FROM gcr.io/distroless/base-debian12
USER nonroot:nonroot
WORKDIR /srv
COPY --from=builder /app/fetcher /srv/fetcher
EXPOSE 8080
ENTRYPOINT ["/srv/fetcher"]
