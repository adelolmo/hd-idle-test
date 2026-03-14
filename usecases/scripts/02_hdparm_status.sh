#!/bin/sh -e

ID="02"
NAME="hdparm power status check spins up disk, but then spins down after 10 minutes"
printf '* %s\n' "$NAME"

# start recording
printf '  Start recording\r'
curl -X POST -H 'Content-Type: application/json' \
  --data '{"name":"'"${ID}"'","action":"start"}' \
  --unix-socket /tmp/hdtd.sock \
  "http://unix/record"

printf '\e[2K\r  Sleeping 11s\r'
sleep 11

printf '\e[2K\r  Sleeping 12m\r'
sleep 720

printf '\e[2K\r  invoking hdparm\r'
sudo hdparm -C /dev/sda > /dev/null 2>&1

printf '\e[2K\r  Sleeping 12m after invoking hdparm\r'
sleep 720

# assert sda is spun down
printf '\e[2K\r  Checking /dev/sda power\r'

up=$(curl -sX GET --unix-socket /tmp/spd.sock "http://unix/devices/sda" |jq .up)
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
