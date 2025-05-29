# Use official Go image
FROM golang:1.23.2-bullseye

# Install required tools for PDF manipulation
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        qpdf \
        unoconv \
        libreoffice \
        curl \
        ghostscript \
        poppler-utils \
        fonts-dejavu \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Set environment variables for non-GUI LibreOffice
ENV HOME=/tmp \
    LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8

# Create working directory
WORKDIR /app

# Copy app files
COPY . .

# Download Go modules early for caching
RUN go mod download

# Build Go binary
RUN go build -o server .

# Expose port
EXPOSE 8080

# Start the app
CMD ["./server"]
