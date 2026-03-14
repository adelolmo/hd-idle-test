#!/bin/sh -e

ID="01"
NAME="Single disk partition spins down after 10 minutes"
printf '* %s\n' "$NAME"

# start recording
printf '  Start recording\r'
curl -X POST -H 'Content-Type: application/json' \
  --data '{"name":"'"${ID}"'","action":"start"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"
sleep 1

# sleeping 11s
printf '\e[2K\r  Sleeping 11s\r'
sleep 11

# write on
printf '\e[2K\r  Write on /mnt/one\r'
date +"%Y-%m-%d %H:%M" > "/mnt/one/$(date +"%Y%m%d-%H%M").txt"

# sleep 12m
printf '\e[2K\r  Sleeping 12m\r'
sleep 720

# checking
printf '\e[2K\r  Checking /dev/sdb power\r'

up=$(curl -sX GET --unix-socket /tmp/spd.sock "http://unix/devices/sdb" |jq .up)
printf '\e[2K\r'
if [ $up = "true" ]; then
  printf '\e[1A\r* %s \033[0;31mFail\033[0m\n' "$NAME"
else
  printf '\e[1A\r* %s \033[0;32mOK\033[0m\n' "$NAME"
fi

sleep 11

# stop recording
curl -X POST -H 'Content-Type: application/json' \
  --data '{"name":"'"${ID}"'","action":"stop"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"
