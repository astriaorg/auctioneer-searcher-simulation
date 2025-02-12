# Build stage: use the official Go image to build the binary.
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container.
WORKDIR /app

# Cache dependencies by copying go.mod and go.sum first.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your application code.
COPY . .

# Build the Go application with CGO disabled for a static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o myapp .

# Final stage: use a minimal base image.
FROM scratch

# Copy the built binary from the builder stage.
COPY --from=builder /app/myapp /myapp

# Command to run your Go binary.
ENTRYPOINT ["/myapp"]