# CA certificates and user from Ubuntu (multi-stage; binary is injected by Goreleaser)
FROM ubuntu:24.04 AS ca-certs
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
RUN groupadd -r -g 10001 dzsa-sync && useradd -r -u 10001 -g dzsa-sync -s /sbin/nologin -d /nonexistent dzsa-sync

# Final image: scratch + binary + CA certs
FROM scratch
COPY --from=ca-certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=ca-certs /etc/passwd /etc/passwd
COPY --from=ca-certs /etc/group /etc/group
COPY dzsa-sync /dzsa-sync
USER dzsa-sync
ENTRYPOINT ["/dzsa-sync"]
