#!/usr/bin/env bash
# Run from directory containing ./raft-node binary. Fresh cluster: rm -rf data logs first.
set -euo pipefail
PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.1:8003,node4=127.0.0.1:8004,node5=127.0.0.1:8005"
mkdir -p logs
for i in 1 2 3 4 5; do
  extra=()
  [[ "$i" == "1" ]] && extra=(-bootstrap)
  ./raft-node -id "node$i" -raft-bind "127.0.0.1:800$i" -http "127.0.0.1:900$i" \
    -peers "$PEERS" "${extra[@]}" >> "logs/node$i.log" 2>&1 &
done
echo "started 5 nodes; curl 127.0.0.1:9001/status"
