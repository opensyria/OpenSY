# OpenSY GPU Miner for Vast.ai
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    git \
    wget \
    curl \
    libboost-all-dev \
    libevent-dev \
    libssl-dev \
    libsqlite3-dev \
    pkg-config \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Create opensy user
RUN useradd -m -s /bin/bash opensy

# Set working directory
WORKDIR /home/opensy

# Download and extract OpenSY binaries (will be built from source)
# For now, we'll clone and build
RUN git clone https://github.com/opensyria/OpenSY.git /home/opensy/OpenSY

WORKDIR /home/opensy/OpenSY

# Build OpenSY
RUN cmake -B build -DBUILD_DAEMON=ON -DBUILD_CLI=ON -DBUILD_TESTS=OFF -DBUILD_GUI=OFF \
    && cmake --build build -j$(nproc)

# Create data directory
RUN mkdir -p /home/opensy/.opensy

# Copy startup script
COPY start-mining.sh /home/opensy/start-mining.sh
RUN chmod +x /home/opensy/start-mining.sh

# Set ownership
RUN chown -R opensy:opensy /home/opensy

USER opensy
WORKDIR /home/opensy

# Expose P2P port
EXPOSE 9633

# Default command
CMD ["/home/opensy/start-mining.sh"]
