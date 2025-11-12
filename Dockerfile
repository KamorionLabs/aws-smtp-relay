# Production image using Distroless (minimal, secure, no shell)
FROM gcr.io/distroless/static-debian12:nonroot

# GoReleaser v2 automatically organizes binaries by platform
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/aws-smtp-relay /bin/aws-smtp-relay

ENTRYPOINT ["/bin/aws-smtp-relay"]
