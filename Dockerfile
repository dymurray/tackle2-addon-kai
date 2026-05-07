# --- Stage 1: Build Go binaries ---
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS addon
ENV GOPATH=$APP_ROOT
COPY --chown=1001:0 . .
RUN make cmd

# --- Stage 2: Build pallet from source as a static (musl) binary.
# The upstream pallet-linux-amd64 release asset is dynamically linked against
# glibc >= 2.39 (Ubuntu 24.04 build host), which is too new for ubi9
# (glibc 2.34). Pallet uses rustls (no native OpenSSL), so it compiles cleanly
# for x86_64-unknown-linux-musl into a fully static binary.
FROM docker.io/library/rust:1.83 AS pallet-builder
ARG PALLET_VERSION=0.0.5
RUN apt-get update && apt-get install -y --no-install-recommends musl-tools \
 && rm -rf /var/lib/apt/lists/* \
 && rustup target add x86_64-unknown-linux-musl
WORKDIR /src
RUN curl -fsSL "https://github.com/djzager/pallet/archive/refs/tags/v${PALLET_VERSION}.tar.gz" \
      | tar -xz --strip-components=1
RUN cargo build --release --target x86_64-unknown-linux-musl

# --- Stage 3: Runtime ---
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# System packages
RUN echo -e "[centos9]" \
 "\nname = centos9" \
 "\nbaseurl = http://mirror.stream.centos.org/9-stream/AppStream/\$basearch/os/" \
 "\nenabled = 1" \
 "\ngpgcheck = 0" > /etc/yum.repos.d/centos.repo
RUN microdnf -y install \
 glibc-langpack-en \
 openssh-clients \
 subversion \
 git \
 tar \
 libxcb
RUN sed -i 's/^LANG=.*/LANG="en_US.utf8"/' /etc/locale.conf
ENV LANG=en_US.utf8

# Install goose and opencode from release binaries; pallet comes from
# the pallet-builder stage (built static against musl).
ARG GOOSE_VERSION=1.23.2
ARG OPENCODE_VERSION=0.0.55
RUN microdnf -y install bzip2 && \
    curl -fsSL -L "https://github.com/block/goose/releases/download/v${GOOSE_VERSION}/goose-x86_64-unknown-linux-gnu.tar.bz2" \
      | tar -xj -C /usr/bin --strip-components=1 && \
    curl -fsSL -L "https://github.com/opencode-ai/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-x86_64.tar.gz" \
      | tar -xz -C /usr/bin opencode && \
    microdnf -y remove bzip2 && microdnf clean all

COPY --from=pallet-builder /src/target/x86_64-unknown-linux-musl/release/pallet /usr/bin/pallet
RUN chmod +x /usr/bin/pallet

# Addon user
RUN echo "addon:x:1001:1001:addon user:/addon:/sbin/nologin" >> /etc/passwd
RUN echo -e "StrictHostKeyChecking no" \
 "\nUserKnownHostsFile /dev/null" > /etc/ssh/ssh_config.d/99-konveyor.conf

# Copy Go binaries from build stage
ARG GOPATH=/opt/app-root
COPY --from=addon $GOPATH/src/bin/addon /usr/bin/addon
COPY --from=addon $GOPATH/src/bin/fetch-analysis /usr/bin/fetch-analysis

# Copy bundled skills
COPY skills/ /addon/skills/

ENV HOME=/addon ADDON=/addon
WORKDIR /addon
USER 1001

ENTRYPOINT ["/usr/bin/addon"]
