# --- Stage 1: Build Go binaries ---
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS addon
ENV GOPATH=$APP_ROOT
COPY --chown=1001:0 . .
RUN make cmd

# --- Stage 2: Runtime ---
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
 tar
RUN sed -i 's/^LANG=.*/LANG="en_US.utf8"/' /etc/locale.conf
ENV LANG=en_US.utf8

# Install goose, opencode, and pallet from release binaries
ARG GOOSE_VERSION=1.23.2
ARG OPENCODE_VERSION=0.0.55
ARG PALLET_VERSION=0.0.5
RUN microdnf -y install bzip2 && \
    curl -fsSL -L "https://github.com/block/goose/releases/download/v${GOOSE_VERSION}/goose-x86_64-unknown-linux-gnu.tar.bz2" \
      | tar -xj -C /usr/bin goose && \
    curl -fsSL -L "https://github.com/opencode-ai/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-x86_64.tar.gz" \
      | tar -xz -C /usr/bin opencode && \
    curl -fsSL -L "https://github.com/djzager/pallet/releases/download/v${PALLET_VERSION}/pallet-linux-amd64" \
      -o /usr/bin/pallet && chmod +x /usr/bin/pallet && \
    microdnf -y remove bzip2 && microdnf clean all

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
