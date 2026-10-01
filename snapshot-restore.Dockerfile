# Init container image for snapshot bootstrap (internal/snapshot). Published by
# .github/workflows/release.yml as ghcr.io/tazhate/chainplane/snapshot-restore:<tag>.
# Base is pinned by digest and pip packages by version so a given release tag
# always ships the same bytes; bump both deliberately.
FROM python:3.11-slim@sha256:e41613d42d4891e4930f79523f93f81bbc7632584ec65e36ab055f41a800b41e
RUN pip install --no-cache-dir \
    minio==7.2.20 \
    requests==2.34.2 \
    lz4==4.4.5
