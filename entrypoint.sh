#!/usr/bin/env sh

if [ "$SET_CONTAINER_TIMEZONE" = "true" ]; then
    echo "$CONTAINER_TIMEZONE" > /etc/timezone \
    && ln -sf "/usr/share/zoneinfo/$CONTAINER_TIMEZONE" /etc/localtime
    echo "Container timezone set to: $CONTAINER_TIMEZONE"
else
    echo "Container timezone not modified"
fi

addgroup -g $USER_GID user
adduser -h /home/user -G user -D -u $USER_UID user

mkdir -p "$ERIN_OLD_MOVE_TO_PATH"

chown -R user:user "$ERIN_OLD_MOVE_TO_PATH"

su-exec user /usr/local/bin/erin &
child=$!

trap "kill $child" SIGTERM SIGINT
wait "$child"
trap - SIGTERM SIGINT
wait "$child"
