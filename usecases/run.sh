#!/bin/sh -e

echo
echo " ┌────────────────────────┐"
echo " │ hd-idle test scenarios │"
echo " └────────────────────────┘"
echo
find scripts -type f -name "*.sh" | sort | xargs bash