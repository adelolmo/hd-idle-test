#!/bin/sh -e

NAME="No disk activity spins down after 10 minutes"
printf '* %s\r' "$NAME"

# single disk partition

# start recording
curl -X POST -H 'Content-Type: application/json' \
  --data '{"action":"start"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"

# wait 10s
sleep 10

# write on
date +"%Y-%m-%d %H:%M" > "/mnt/one/$(date +"%Y%m%d-%H%M").txt"

# wait 10m 5s
sleep 605

# assert sdb is spun down
printf '* %s \033[0;31mFail\033[0m\r\n' "$NAME"
printf '* %s \033[0;32mOK\033[0m\r\n' "$NAME"

# wait 10s
sleep 10

# stop recording
curl -X POST -H 'Content-Type: application/json' \
  --data '{"action":"stop"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"