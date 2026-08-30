FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git nodejs npm openssl sudo && \
    rm -rf /var/lib/apt/lists/*

ARG PI_CODING_AGENT_PACKAGE=@earendil-works/pi-coding-agent@0.84.3
ARG PI_MCP_ADAPTER_PACKAGE=pi-mcp-adapter@2.27.0
ARG PI_WEB_ACCESS_TARBALL=https://registry.npmjs.org/pi-web-access/-/pi-web-access-0.24.2.tgz
ARG PI_WEB_ACCESS_SHA512=c29c355c92c84eb2d8e5ea590a3886c5d17be4794a8194eb2b550cdd31bf01547346b16abcda1ce1b2d86821458d8506f10344a3a748a586c227201490bcff40
RUN npm install -g node@22.19.0 && \
    node --version && \
    npm install -g "${PI_CODING_AGENT_PACKAGE}" && \
    npm install --prefix /opt/pi-extensions/pi-mcp-adapter "${PI_MCP_ADAPTER_PACKAGE}" --legacy-peer-deps && \
    curl -fsSL "${PI_WEB_ACCESS_TARBALL}" -o /tmp/pi-web-access.tgz && \
    printf '%s  %s\n' "${PI_WEB_ACCESS_SHA512}" /tmp/pi-web-access.tgz | sha512sum -c - && \
    npm install --prefix /opt/pi-extensions/pi-web-access /tmp/pi-web-access.tgz --legacy-peer-deps --ignore-scripts && \
    rm /tmp/pi-web-access.tgz && \
    chmod -R a+rX /opt/pi-extensions && \
    npm cache clean --force

RUN useradd -m user && echo 'user ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/user
USER user
WORKDIR /home/user/work
ENTRYPOINT ["pi"]
