# Use a base image with Go
FROM golang:1.21-bullseye

# Install qpdf
RUN apt-get update && apt-get install -y qpdf

# Create app directory
WORKDIR /app

# Copy Go files
COPY . .

# Build your Go app
RUN go build -o server .

# Expose your port
EXPOSE 8080

# Run your app
CMD ["./server"]
