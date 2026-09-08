# The x402 end-to-end harness.
#
# Built with the repository root as context, so the binary under test is
# compiled from this tree inside the image rather than carried in from a host
# that may have built it differently:
#
#   docker build -f scripts/x402-e2e/Dockerfile -t replay-x402-e2e .
#   docker run --rm replay-x402-e2e
#
# scripts/x402-e2e/e2e.sh does both and is what CI runs.
FROM golang:1.24-bookworm

WORKDIR /src
COPY . /src/
RUN go build -trimpath \
      -ldflags "-X github.com/RedRobotKK/Replay/internal/version.Version=x402-e2e" \
      -o /usr/local/bin/replay ./cmd/replay && \
    go build -o /usr/local/bin/seller ./scripts/x402-e2e

# A CA and a server certificate for seller.test, with the CA installed into the
# container's trust store.
#
# Replay refuses plain http and verifies the chain, which is the behaviour under
# test. Disabling verification to make the harness simpler would have tested a
# different program, so the certificate is made genuinely trusted instead. One
# day of validity: this image is built per run and a long-lived certificate in a
# public repository is a habit worth not forming.
RUN mkdir -p /certs && \
    openssl req -x509 -newkey rsa:2048 -nodes -keyout /certs/ca.key -out /certs/ca.crt \
      -days 1 -subj "/CN=x402-e2e-ca" >/dev/null 2>&1 && \
    openssl req -newkey rsa:2048 -nodes -keyout /certs/server.key -out /certs/server.csr \
      -subj "/CN=seller.test" >/dev/null 2>&1 && \
    printf "subjectAltName=DNS:seller.test\n" > /certs/ext.cnf && \
    openssl x509 -req -in /certs/server.csr -CA /certs/ca.crt -CAkey /certs/ca.key \
      -CAcreateserial -out /certs/server.crt -days 1 -extfile /certs/ext.cnf >/dev/null 2>&1 && \
    cp /certs/ca.crt /usr/local/share/ca-certificates/x402-e2e.crt && \
    update-ca-certificates >/dev/null 2>&1

WORKDIR /work
# /etc/hosts is a runtime mount and cannot be written at build time, so the name
# is added when the container starts.
CMD ["/bin/bash", "-c", "echo '127.0.0.1 seller.test' >> /etc/hosts; seller 2>/tmp/seller.log & sleep 1; /src/scripts/x402-e2e/run.sh"]
