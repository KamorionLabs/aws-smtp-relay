# Production image using Distroless (minimal, secure, no shell)
FROM gcr.io/distroless/static-debian12:nonroot

# GoReleaser v2 organizes binaries in $TARGETPLATFORM/ (e.g., linux/amd64/)
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/aws-smtp-relay /bin/aws-smtp-relay

ENTRYPOINT ["/bin/aws-smtp-relay"]
