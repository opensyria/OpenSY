#!/bin/bash
DATADIR="$HOME/Library/Application Support/OpenSyria"
CLI="/Users/hamoudi/OpenSyria/build_regular/bin/opensy-cli -datadir=\"$DATADIR\""
ADDR="syl1q0y76xxxdfvhfad2sju4fymnsn8zs5lndpwhufw"

while true; do
    echo "$(date): Mining batch of 10 blocks..."
    eval $CLI generatetoaddress 10 $ADDR
    sleep 1
done
