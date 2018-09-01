FROM golang:1.10.3-alpine3.8 AS build

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
  && go install

FROM alpine:3.8

RUN \
  apk add --no-cache \
    libpcap \
    su-exec \
    tzdata

ENV \
  USER_UID=1000 \
  USER_GID=1000 \
  \
  SET_CONTAINER_TIMEZONE=true \
  CONTAINER_TIMEZONE=Europe/Moscow \
  \
  ERIN_IN_DUMP_PATH= \
  ERIN_OLD_MOVE_TO_PATH= \
  ERIN_TEMP_STORE_PATH= \
  ERIN_RABBITMQ_USER= \
  ERIN_RABBITMQ_PASSWORD= \
  ERIN_RABBITMQ_ADDR= \
  ERIN_RABBITMQ_VHOST=

COPY --from=build /go/bin/erin /usr/local/bin/erin
COPY etc /etc/
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

CMD ["/usr/local/bin/entrypoint.sh"]
