FROM golang:1.23-alpine AS builder

# Set build context to the entire blueprint project
WORKDIR /blueprint

# Copy entire project
COPY . .

# Build from the etcd sample directory
WORKDIR /blueprint/samples/etcd

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o etcd-sample main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /blueprint/samples/etcd/etcd-sample .

# Make it executable
RUN chmod +x ./etcd-sample

# Command to run
CMD ["./etcd-sample"]