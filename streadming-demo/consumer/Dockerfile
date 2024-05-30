FROM golang:1.19-alpine

# Set the working directory
WORKDIR /app

# Copy source code
COPY . .

# Build the Go application
RUN go mod init thiago.io/consumer
RUN go get github.com/go-redis/redis/v8
RUN go get github.com/google/uuid
RUN go mod download
RUN go build -o main .

# Command to run the executable
CMD ["./main"]

