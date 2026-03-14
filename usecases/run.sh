#!/bin/sh

pid=$(pidof hd-idle)

if [ "$?" -eq 1 ]; then
  echo "Error! hd-idle is not running"
  echo "Run the command to start:"
  echo "  # touch /var/log/hd-idle.log && nohup /usr/sbin/hd-idle -l /var/log/hd-idle.log > /tmp/hd-idle.out 2>&1 &"
  exit 1
fi

echo
echo "hd-idle pid: $pid"
echo
echo " ┌────────────────────────┐"
echo " │ hd-idle test scenarios │"
echo " └────────────────────────┘"
echo

for f in scripts/*.sh; do
  bash "$f"
done