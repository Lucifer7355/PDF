# Use an official Golang image as base
FROM golang:1.23.2-bullseye

# Install qpdf, unoconv, libreoffice for PDF conversion & security
RUN apt-get update && apt-get install -y \
    qpdf \
    unoconv \
    libreoffice \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy app files
COPY . .

# Build Go binary
RUN go build -o server .

# Expose port
EXPOSE 8080

# Start app
CMD ["./server"]
