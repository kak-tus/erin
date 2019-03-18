FROM golang:1.12.0-alpine3.9 AS build

WORKDIR /go/src/github.com/kak-tus/erin

COPY founder ./founder
COPY parser ./parser
COPY vendor ./vendor
COPY main.go .

RUN \
  apk add --no-cache \
    build-base \
    libpcap-dev \
  \
  && go build -o /go/bin/erin

FROM alpine:3.9

RUN \
  apk add --no-cache \
    libpcap \
    su-exec \
    tzdata

ENV \
  USER_GID=1000 \
  USER_UID=1000 \
  \
  CONTAINER_TIMEZONE=Europe/Moscow \
  SET_CONTAINER_TIMEZONE=true \
  \
  ERIN_IN_DUMP_PATH= \
  ERIN_OLD_MOVE_TO_PATH= \
  ERIN_PENDING_BUFFER_SIZE=1000000 \
  ERIN_PIPE_BUFFER_SIZE=50000 \
  ERIN_REDIS_ADDRS= \
  ERIN_REDIS_PASSWORD= \
  ERIN_SHARDS_COUNT=10

COPY --from=build /go/bin/erin /usr/local/bin/erin
COPY etc /etc/
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

CMD ["/usr/local/bin/entrypoint.sh"]
