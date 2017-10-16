FROM golang:1.9-alpine3.6 AS builder

RUN apk -v --update-cache --no-progress add \
  bash \
  git \
  make \
  && rm /var/cache/apk/*

RUN go get -u github.com/alecthomas/gometalinter \
  && gometalinter --install

WORKDIR /go/src/github.com/saj/vault-auto-unseal

COPY GNUmakefile .
COPY scripts scripts/
COPY vendor vendor/
COPY *.go ./

RUN make lint
RUN make test
RUN make vault-auto-unseal-linux-amd64


FROM alpine:3.6

WORKDIR /root

COPY entrypoint /usr/local/bin/entrypoint

COPY --from=builder \
  /go/src/github.com/saj/vault-auto-unseal/build/linux_amd64/vault-auto-unseal \
  /usr/local/bin/vault-auto-unseal

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD ["--help"]
