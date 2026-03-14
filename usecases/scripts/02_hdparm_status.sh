#!/bin/sh -e

NAME="hdparm power status check spins up disk, but then spins down after 10 minutes"
printf '* %s\r' "$NAME"

# start recording
curl -X POST -H 'Content-Type: application/json' \
  --data '{"action":"start"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"

sleep 11

# sleep 12m
sleep 720

sudo hdparm -C /dev/sdb

# sleep 12m
sleep 720

# assert sdb is spun down
up=$(curl -sX GET --unix-socket /tmp/spd.sock "http://unix/devices/sdb" |jq .up)
if [ $up = "true" ]; then
  printf '* %s \033[0;31mFail\033[0m\r\n' "$NAME"
else
  printf '* %s \033[0;32mOK\033[0m\r\n' "$NAME"
fi

sleep 11

# stop recording
curl -X POST -H 'Content-Type: application/json' \
  --data '{"action":"stop"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"
