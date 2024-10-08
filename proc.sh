#!/usr/bin/env bash

i=0

while [ $i != 4 ]; do
    if [ $((i%3)) == 0 ]; then
        echo -n "tick: $i"
    else
        echo "tick: $i"
    fi

    sleep 1s
    i=$((i+1))
done

# echo "stderr" >&2
# echo "stdout" >&1

# env

# exit 2
