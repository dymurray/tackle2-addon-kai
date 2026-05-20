# --- Stage 1: Build Go binaries ---
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS addon
ENV GOPATH=$APP_ROOT
COPY --chown=1001:0 . .
RUN make cmd

# --- Stage 2: Build pallet on UBI9 so it links against the same glibc (2.34)
# as the runtime image. The upstream release asset targets glibc >= 2.39.
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS pallet-builder
ARG PALLET_VERSION=0.0.5
RUN echo -e "[centos9-appstream]" \
 "\nname = CentOS Stream 9 - AppStream" \
 "\nbaseurl = http://mirror.stream.centos.org/9-stream/AppStream/\$basearch/os/" \
 "\nenabled = 1" \
 "\ngpgcheck = 0" \
 "\n" \
 "\n[centos9-baseos]" \
 "\nname = CentOS Stream 9 - BaseOS" \
 "\nbaseurl = http://mirror.stream.centos.org/9-stream/BaseOS/\$basearch/os/" \
 "\nenabled = 1" \
 "\ngpgcheck = 0" > /etc/yum.repos.d/centos.repo
RUN microdnf -y install rust cargo gcc make tar && microdnf clean all
WORKDIR /src
RUN curl -fsSL "https://github.com/djzager/pallet/archive/refs/tags/v${PALLET_VERSION}.tar.gz" \
      | tar -xz --strip-components=1
RUN cargo build --release

# --- Stage 3: Runtime ---
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# System packages
RUN echo -e "[centos9-appstream]" \
 "\nname = CentOS Stream 9 - AppStream" \
 "\nbaseurl = http://mirror.stream.centos.org/9-stream/AppStream/\$basearch/os/" \
 "\nenabled = 1" \
 "\ngpgcheck = 0" \
 "\n" \
 "\n[centos9-baseos]" \
 "\nname = CentOS Stream 9 - BaseOS" \
 "\nbaseurl = http://mirror.stream.centos.org/9-stream/BaseOS/\$basearch/os/" \
 "\nenabled = 1" \
 "\ngpgcheck = 0" > /etc/yum.repos.d/centos.repo
RUN microdnf -y install \
 glibc-langpack-en \
 openssh-clients \
 subversion \
 git \
 tar \
 libxcb \
 gcc \
 gcc-c++ \
 make \
 patch \
 go-toolset \
 nodejs \
 npm \
 python3 \
 python3-pip \
 python3-devel \
 python3-setuptools \
 java-17-openjdk-devel \
 maven \
 dotnet-sdk-8.0 \
 rust \
 cargo \
 && microdnf clean all
RUN sed -i 's/^LANG=.*/LANG="en_US.utf8"/' /etc/locale.conf
ENV LANG=en_US.utf8

# TypeScript (via npm) and Python convenience symlink
RUN npm install -g typescript && \
    ln -sf /usr/bin/python3 /usr/bin/python

# Language toolchain environment
ENV JAVA_HOME=/usr/lib/jvm/java-17-openjdk
ENV DOTNET_CLI_TELEMETRY_OPTOUT=1
ENV DOTNET_NOLOGO=1

# Install goose and opencode from release binaries.
ARG GOOSE_VERSION=1.23.2
ARG OPENCODE_VERSION=0.0.55
RUN microdnf -y install bzip2 && \
    curl -fsSL -L "https://github.com/block/goose/releases/download/v${GOOSE_VERSION}/goose-x86_64-unknown-linux-gnu.tar.bz2" \
      | tar -xj -C /usr/bin --strip-components=1 && \
    curl -fsSL -L "https://github.com/opencode-ai/opencode/releases/download/v${OPENCODE_VERSION}/opencode-linux-x86_64.tar.gz" \
      | tar -xz -C /usr/bin opencode && \
    microdnf -y remove bzip2 && microdnf clean all

COPY --from=pallet-builder /src/target/release/pallet /usr/bin/pallet
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

COPY goosehints/mempalace /usr/share/goosehints

ENV HOME=/addon ADDON=/addon
WORKDIR /addon
USER 1001

ENTRYPOINT ["/usr/bin/addon"]
