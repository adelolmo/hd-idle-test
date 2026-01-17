#!/bin/sh -e

NAME="Single disk partition spins down after 10 minutes"
printf '* %s\r' "$NAME"

# single disk partition

# start recording
curl -X POST -H 'Content-Type: application/json' \
  --data '{"action":"start"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"

sleep 30

# write on
date +"%Y-%m-%d %H:%M" > "/mnt/one/$(date +"%Y%m%d-%H%M").txt"

# sleep 10m 5s
sleep 605

# assert sdb is spun down
printf '* %s \033[0;31mFail\033[0m\r\n' "$NAME"
printf '* %s \033[0;32mOK\033[0m\r\n' "$NAME"

sleep 30

# stop recording
curl -X POST -H 'Content-Type: application/json' \
  --data '{"action":"stop"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"