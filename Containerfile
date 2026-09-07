# Build stage
FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o self-ip .

FROM alpine:latest
WORKDIR /app
# Copy the compiled binary
COPY --from=builder /app/self-ip /usr/bin/

# Application port
EXPOSE 3213

# Run the application
CMD ["/usr/bin/self-ip"]
