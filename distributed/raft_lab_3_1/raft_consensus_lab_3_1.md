# Lab 3.1 — Raft Leader Election & Log Replication

Build a **5-node Raft cluster** on one **GCP Ubuntu VM** (peers on `127.0.0.1:8001`–`8005`, HTTP `9001`–`9005`). Measure **leader failover**, **partition 2|3**, and produce a **term/log diagram** after heal.

**Laptop:** `gcloud` create / SSH / delete only. **VM:** clone repo, build, run all lab commands.

Implementation: **`cmd/raft-node/main.go`** ([HashiCorp `raft`](https://github.com/hashicorp/raft)).

---

## GCP VM

| Item | Choice |
|------|--------|
| Machine | **e2-standard-4** (or **e2-standard-2** for dev) |
| OS | Ubuntu **22.04** x86_64, 30 GB disk |

```bash
# Laptop
gcloud config set project YOUR_PROJECT_ID

gcloud compute instances create lab-raft-31 \
  --zone=us-central1-a \
  --machine-type=e2-standard-4 \
  --image-family=ubuntu-2204-lts \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=30GB

gcloud compute ssh lab-raft-31 --zone=us-central1-a

# When done
gcloud compute instances delete lab-raft-31 --zone=us-central1-a --quiet
```

---

## Setup on the VM (clone + build)

```bash
sudo apt update
sudo apt install -y git jq iptables iproute2 curl

# Not apt golang-go (Go 1.18 on 22.04)
GOVER=1.22.10
curl -fsSL "https://go.dev/dl/go${GOVER}.linux-amd64.tar.gz" -o /tmp/go.tgz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tgz
grep -q '/usr/local/go/bin' ~/.bashrc || echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
export PATH=/usr/local/go/bin:$PATH

git clone https://github.com/YOUR_USER/systemlab.git
export LAB_DIR=~/systemlab/distributed/raft_lab_3_1
cd "$LAB_DIR"

go build -o raft-node ./cmd/raft-node
mkdir -p logs data
```

Updates: `cd ~/systemlab && git pull && cd "$LAB_DIR" && go build -o raft-node ./cmd/raft-node`  
Fresh cluster: `rm -rf data logs` (then **`-bootstrap` on node1 only**).

---

## Step 1 — Start 5-node cluster

```bash
cd "$LAB_DIR"

PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.1:8003,node4=127.0.0.1:8004,node5=127.0.0.1:8005"
rm -rf data logs && mkdir -p logs

./raft-node -id node1 -raft-bind 127.0.0.1:8001 -http 127.0.0.1:9001 -peers "$PEERS" -bootstrap \
  >> logs/node1.log 2>&1 &
for i in 2 3 4 5; do
  ./raft-node -id "node$i" -raft-bind "127.0.0.1:800$i" -http "127.0.0.1:900$i" -peers "$PEERS" \
    >> "logs/node$i.log" 2>&1 &
done

sleep 2
for p in 9001 9002 9003 9004 9005; do curl -s "127.0.0.1:$p/status" | jq .; done
```

**Pass:** one `"leader": true`, `/write` on that node’s HTTP port succeeds.

```bash
# Baseline writes (use leader port)
for i in $(seq 1 100); do curl -sf "127.0.0.1:9001/write?key=k$i&val=v$i" || break; done
```

---

## Step 2 — Kill leader, measure re-election

```bash
cd "$LAB_DIR"
PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.1:8003,node4=127.0.0.1:8004,node5=127.0.0.1:8005"

LEADER=""
for p in 9001 9002 9003 9004 9005; do
  if curl -sf "127.0.0.1:$p/status" | jq -e '.leader == true' >/dev/null; then
    LEADER=$(curl -s "127.0.0.1:$p/status" | jq -r '.id')
    break
  fi
done
echo "leader=$LEADER"

T0=$(date +%s%N)
kill $(pgrep -f "raft-node -id $LEADER")

# Poll /status until a new leader (note term + id); then:
# T1=... ; echo "election_ms=$(( (T1 - T0) / 1000000 ))"
```

Record: old/new leader, term, **election_ms**, split-vote signs in `logs/`.

**Bring the killed node back** (same `-id` / ports as Step 1, **no `-bootstrap`** — it rejoins from `./data/<id>/`):

```bash
cd "$LAB_DIR"
PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.1:8003,node4=127.0.0.1:8004,node5=127.0.0.1:8005"

# Example: you killed node1 — restart node1 only
./raft-node -id node1 -raft-bind 127.0.0.1:8001 -http 127.0.0.1:9001 -peers "$PEERS" \
  >> logs/node1.log 2>&1 &

sleep 2
curl -s 127.0.0.1:9001/status | jq .    # expect role follower, term matches cluster
```

For any other node, change `node1`, `8001`, `9001`, and `logs/node1.log`. The cluster keeps running; the restarted process catches up via Raft replication.

---

## Step 3 — Partition 2 | 3, then heal

**Minority** node1–2 vs **majority** node3–5. On one VM, Raft dials use **ephemeral TCP ports**, so `sport 8001` iptables rules **do not block** traffic (DROP counters stay 0). Use **two loopback IPs** for Raft, then drop **127.0.0.1 ↔ 127.0.0.2** on `lo`.

**3a — Start cluster with split Raft addresses** (fresh `data/`):

```bash
cd "$LAB_DIR"
pkill -f 'raft-node -id' || true
sudo iptables -F INPUT
rm -rf data logs && mkdir -p logs

PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.2:8003,node4=127.0.0.2:8004,node5=127.0.0.2:8005"

./raft-node -id node1 -raft-bind 127.0.0.1:8001 -http 127.0.0.1:9001 -peers "$PEERS" -bootstrap >> logs/node1.log 2>&1 &
./raft-node -id node2 -raft-bind 127.0.0.1:8002 -http 127.0.0.1:9002 -peers "$PEERS" >> logs/node2.log 2>&1 &
./raft-node -id node3 -raft-bind 127.0.0.2:8003 -http 127.0.0.1:9003 -peers "$PEERS" >> logs/node3.log 2>&1 &
./raft-node -id node4 -raft-bind 127.0.0.2:8004 -http 127.0.0.1:9004 -peers "$PEERS" >> logs/node4.log 2>&1 &
./raft-node -id node5 -raft-bind 127.0.0.2:8005 -http 127.0.0.1:9005 -peers "$PEERS" >> logs/node5.log 2>&1 &

sleep 3
for p in 9001 9002 9003 9004 9005; do curl -s "127.0.0.1:$p/status" | jq -c '{id,leader,term}'; done
```

**3b — Apply partition** (while all five processes run):

```bash
sudo iptables -A INPUT -i lo -s 127.0.0.1 -d 127.0.0.2 -p tcp -j DROP
sudo iptables -A INPUT -i lo -s 127.0.0.2 -d 127.0.0.1 -p tcp -j DROP
sudo iptables -L INPUT -n -v --line-numbers   # pkts should increase on rules 1–2

sleep 5
for p in 9001 9002 9003 9004 9005; do echo -n "$p: "; curl -s "127.0.0.1:$p/status" | jq -c '{id,leader,term,commitIndex}'; done

curl -sf "127.0.0.1:9003/write?key=maj&val=1"; echo || echo "majority write failed"
curl -sf "127.0.0.1:9001/write?key=min&val=1"; echo || echo "minority rejected (expected)"
```

**3c — Heal** (lab VM only):

```bash
sudo iptables -F INPUT
sleep 3
for p in 9001 9002 9003 9004 9005; do curl -s "127.0.0.1:$p/status" | jq -c '{id,leader,term,commitIndex}'; done
grep -E 'truncate|AppendEntries|conflict' logs/node*.log || true
```

**Deliverable:** term/log diagram for partition + heal; explain truncated minority entries ([Raft paper](https://raft.github.io/raft.pdf) — log matching, majority commit).

Example diagram:

```text
term 5  majority leader C:  [1:5][2:5][3:5][4:5] commit=4
term 4  minority "leader" A: [1:4][2:4][3:4']  (3' uncommitted)
        after heal: A truncates from index 3, copies leader log
```

---

## Step 4 — One follower down (quorum)

```bash
kill $(pgrep -f 'raft-node -id node4')
# Writes still OK with 4/5; repeat until only 2 nodes — writes should fail
```

Compare write success rate to Step 1 baseline.

---

## Report (short)

1. Re-election timing and term change  
2. Partition: commits majority vs minority; per-node `/status`  
3. Heal: truncation / log sync  
4. Throughput with 5 nodes vs one follower stopped  

---

## Pitfalls

| Issue | Fix |
|-------|-----|
| Go 1.18 / `go mod tidy` error | Install Go from [go.dev/dl](https://go.dev/dl/) (see Setup) |
| `missing go.sum entry` | `git pull` (committed `go.sum`) |
| No leader | Check ports 8001–8005; wipe `data/` and restart with `-bootstrap` on node1 only |
| Stale cluster state | `rm -rf data logs` before clean rerun |
| Partitions on Mac | Use the GCP VM (`iptables` on `lo`) |
| DROP rules but `pkts 0`, one leader | **sport 800x rules don’t match Raft TCP** — use Step 3 **127.0.0.1 / 127.0.0.2** split |

---

## Clean reset (kill all + fresh cluster)

**On the VM**, from `$LAB_DIR`:

```bash
export LAB_DIR=~/systemlab/distributed/raft_lab_3_1
cd "$LAB_DIR"

pkill -f 'raft-node -id' || true
sleep 1
sudo iptables -F INPUT
sudo iptables -F OUTPUT
rm -rf data logs
mkdir -p logs

PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.2:8003,node4=127.0.0.2:8004,node5=127.0.0.2:8005"

./raft-node -id node1 -raft-bind 127.0.0.1:8001 -http 127.0.0.1:9001 -peers "$PEERS" -bootstrap >> logs/node1.log 2>&1 &
./raft-node -id node2 -raft-bind 127.0.0.1:8002 -http 127.0.0.1:9002 -peers "$PEERS" >> logs/node2.log 2>&1 &
./raft-node -id node3 -raft-bind 127.0.0.2:8003 -http 127.0.0.1:9003 -peers "$PEERS" >> logs/node3.log 2>&1 &
./raft-node -id node4 -raft-bind 127.0.0.2:8004 -http 127.0.0.1:9004 -peers "$PEERS" >> logs/node4.log 2>&1 &
./raft-node -id node5 -raft-bind 127.0.0.2:8005 -http 127.0.0.1:9005 -peers "$PEERS" >> logs/node5.log 2>&1 &

sleep 3
ss -ltn | grep -E '800[1-5]'
for p in 9001 9002 9003 9004 9005; do echo -n "$p: "; curl -s "127.0.0.1:$p/status" | jq -c '{id,leader,term,commitIndex}'; done
```

Partition when healthy: Step 3b iptables. Heal: `sudo iptables -F INPUT` (+ `OUTPUT` if used).

---

## Quick reference

```bash
export LAB_DIR=~/systemlab/distributed/raft_lab_3_1
cd "$LAB_DIR"
# clean reset: section above | partition: Step 3
```
