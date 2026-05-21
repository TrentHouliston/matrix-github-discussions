FROM golang:1.25-alpine3.21 AS builder

RUN apk add --no-cache git ca-certificates build-base olm-dev

COPY . /build
WORKDIR /build
RUN chmod +x build.sh && ./build.sh && install -m 0755 mautrix-ghdiscussions /usr/bin/mautrix-ghdiscussions

FROM alpine:3.23

ENV UID=1337 \
	GID=1337

RUN apk add --no-cache su-exec ca-certificates olm bash curl

COPY --from=builder /usr/bin/mautrix-ghdiscussions /usr/bin/mautrix-ghdiscussions
COPY docker-run.sh /docker-run.sh
RUN chmod +x /docker-run.sh

VOLUME /data
WORKDIR /data

EXPOSE 29348

CMD ["/docker-run.sh"]
