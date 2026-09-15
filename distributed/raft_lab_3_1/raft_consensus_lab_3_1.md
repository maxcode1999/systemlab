# Lab 3.1 — Consensus from Scratch: Raft Leader Election & Log Replication

Build a **5-node Raft cluster** (five processes on **localhost**), measure **leader failover** and **partition behavior**, and produce a **term/log diagram** for partition-and-heal.

**Tools:** [HashiCorp `raft`](https://github.com/hashicorp/raft) (Go) *or* [etcd `raft`](https://github.com/etcd-io/raft) · **5 processes** · **`iptables`** / **`tc netem`** for partitions  
**Cluster layout:** nodes `127.0.0.1:8001` … `8005` (RPC) + separate metrics/log ports as you choose

**Metrics:** election → new-leader latency, **term** progression, **truncation** of minority uncommitted entries on heal, **write throughput** (steady vs one-node failure)

Run the lab on a **Linux VM** (recommended: **GCP Ubuntu**, same workflow as the huge-pages lab). All five Raft peers bind **127.0.0.1** on the VM; **iptables** partitions **localhost** traffic between ports.

---

## GCP VM (ephemeral)

| Item | Choice |
|------|--------|
| **RAM / CPU** | **e2-standard-4** (4 vCPU, 16 GiB) — comfortable for 5 nodes + load; **e2-standard-2** works for dev |
| **OS** | Ubuntu **22.04 LTS** x86_64 |
| **Disk** | 20–30 GB boot disk (Raft data + logs) |
| **Network** | Raft RPC on **loopback only** — no extra firewall rules for 8001–8005; SSH is default |

```bash
# Default GCP project (once on your laptop)
gcloud config set project YOUR_PROJECT_ID

# Create lab VM
gcloud compute instances create lab-raft-31 \
  --zone=us-central1-a \
  --machine-type=e2-standard-4 \
  --image-family=ubuntu-2204-lts \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=30GB

# SSH into the VM
gcloud compute ssh lab-raft-31 --zone=us-central1-a

# When finished (stops compute + this instance's disk charges)
gcloud compute instances delete lab-raft-31 --zone=us-central1-a --quiet
```

**Recommended:** SSH to the VM, **`git clone`** systemlab, install **Go**, build **`raft-node`** under `distributed/raft_lab_3_1/`. All steps below assume that layout unless you use the optional Mac cross-build path.

### Path A — Clone repo and build on the VM (recommended)

From your laptop, create the VM and SSH in (commands above). **On the VM:**

```bash
# Packages: partitions, JSON status (do NOT use apt golang-go — it is Go 1.18 on Ubuntu 22.04)
sudo apt update
sudo apt install -y git jq iptables iproute2 curl

# Install Go 1.22+ (required by go.mod; matches hashicorp/raft v1.7)
GOVER=1.22.10
curl -fsSL "https://go.dev/dl/go${GOVER}.linux-amd64.tar.gz" -o /tmp/go.tgz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tgz
export PATH=/usr/local/go/bin:$PATH
go version   # expect go1.22.x

# Clone (HTTPS or SSH — use the URL for your fork)
git clone https://github.com/YOUR_USER/systemlab.git
cd systemlab/distributed/raft_lab_3_1

# go.sum is in the repo — build only; run go mod tidy if you change go.mod
go build -o raft-node ./cmd/raft-node
chmod +x raft-node

mkdir -p logs data
```

Lab code paths on the VM:

| Path | Contents |
|------|----------|
| `~/systemlab/distributed/raft_lab_3_1/` | `go.mod`, `cmd/raft-node/main.go`, `scripts/start-5.sh`, this doc |
| `~/systemlab/distributed/raft_lab_3_1/raft-node` | built binary (after `go build`) |
| `~/systemlab/distributed/raft_lab_3_1/data/node*` | per-node BoltDB + snapshots (created at runtime) |
| `~/systemlab/distributed/raft_lab_3_1/logs/` | optional stdout/stderr from background starts |

To pull updates after you push from your laptop:

```bash
cd ~/systemlab
git pull
cd distributed/raft_lab_3_1
go build -o raft-node ./cmd/raft-node
```

**Fresh cluster:** wipe persisted Raft state before re-bootstrap: `rm -rf data logs`.

### Path B — Build on Mac, copy binary only (optional)

Use this if you do not want Go or a full clone on the VM — only the **`raft-node`** binary (and optional **`client-load`**) plus partition tools.

On your **Mac** (repo checkout):

```bash
cd /path/to/systemlab/distributed/raft_lab_3_1
GOOS=linux GOARCH=amd64 go build -o raft-node ./cmd/raft-node

gcloud compute scp raft-node lab-raft-31:~/raft-lab/raft-node --zone=us-central1-a
gcloud compute ssh lab-raft-31 --zone=us-central1-a --command 'chmod +x ~/raft-lab/raft-node'
```

On the VM:

```bash
sudo apt update
sudo apt install -y jq iptables iproute2
mkdir -p ~/raft-lab/logs ~/raft-lab/data
cd ~/raft-lab
# run ./raft-node from here (no git, no go on VM)
```

**Optional:** use **Spot** for cheaper short labs: add `--provisioning-model=SPOT --instance-termination-action=STOP` to `instances create`.

---

## Prerequisites

```bash
# On GCP VM (Path A: repo build)
cd ~/systemlab/distributed/raft_lab_3_1
test -x ./raft-node || { go version && go build -o raft-node ./cmd/raft-node; }
sudo iptables -L -n >/dev/null

# Path B only: test -x ~/raft-lab/raft-node
```

Pick **one** library and one repo layout:

| Option | Package | Notes |
|--------|---------|--------|
| **A (recommended)** | `github.com/hashicorp/raft` | Full node, transport, snapshots; good for “from scratch” wiring |
| **B** | `go.etcd.io/raft/v3` | Lower-level; you implement network/storage glue (more work) |

This doc assumes **HashiCorp Raft** for steps; map the same *phases* to etcd if you choose B.

---

## Step 0 — Minimal node behavior (implement once)

Each node must:

1. **Persist** raft log + stable store (terms/votes) on disk under `./data/node{id}/`.
2. **Expose** TCP transport between peers (lib raft network layer or gRPC).
3. **Apply** committed entries to an in-memory **state machine** (e.g. `map[string]string` or append-only **write counter**).
4. **Expose HTTP** (or log lines) for: `{ "leader": bool, "term": n, "id": "node3", "lastIndex": …, "commitIndex": … }`.
5. **Client API:** `POST /write?key=k&val=v` → propose via `raft.Apply`; `GET /read?key=k` → **linearizable read** (see Step 6).

Log every **state change** with **monotonic local timestamp** (for diagrams only — Raft does **not** use wall clock for correctness):

```text
2026-09-15T12:00:01.234 node3 term=2 role=follower
2026-09-15T12:00:05.891 node3 term=3 role=candidate
2026-09-15T12:00:05.912 node3 term=3 role=leader
```

---

## Step 1 — Bring up 5-node cluster

```bash
# Path A (clone on VM)
cd ~/systemlab/distributed/raft_lab_3_1
# Path B: cd ~/raft-lab

PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.1:8003,node4=127.0.0.1:8004,node5=127.0.0.1:8005"
rm -rf data logs   # first run only — omit if restarting with existing state
mkdir -p logs

# node1 only: -bootstrap seeds membership on empty data dirs (not a separate coordinator)
./raft-node -id node1 -raft-bind 127.0.0.1:8001 -http 127.0.0.1:9001 -peers "$PEERS" -bootstrap \
  >> logs/node1.log 2>&1 &
for i in 2 3 4 5; do
  ./raft-node -id "node$i" -raft-bind "127.0.0.1:800$i" -http "127.0.0.1:900$i" -peers "$PEERS" \
    >> "logs/node$i.log" 2>&1 &
done

# Or: chmod +x scripts/start-5.sh && ./scripts/start-5.sh

# Wait until one leader (all nodes same term, one leader=true)
for p in 9001 9002 9003 9004 9005; do curl -s "127.0.0.1:$p/status" | jq .; done
curl -s "127.0.0.1:9001/write?key=hello&val=world"   # retry on port whose /status shows leader=true
```

**Pass:** exactly **one** leader, **term ≥ 1**, `/write` succeeds on leader HTTP port.

**Steady-state throughput baseline** (leader only):

```bash
# Example: 10s write load (adapt to your client script)
time ./client-load -leader "127.0.0.1:9001" -duration 10s -concurrency 4
# Record: writes/sec, errors
```

---

## Step 2 — Kill leader, time re-election

```bash
# Identify leader
LEADER=$(curl -s 127.0.0.1:9001/status | jq -r '.id')   # repeat on 9002..9005 until .leader==true

# Record start time, kill leader process
T0=$(date +%s%N)
kill $(pgrep -f "raft-node -id $LEADER")   # or kill specific PID

# Poll until a new leader appears (different id, term increased)
# Record T1 when new leader stable for 3 consecutive polls
T1=...
echo "election_ms=$(( (T1 - T0) / 1000000 ))"
```

**Record:** old `(term, leader)` → new `(term, leader)`, **election_ms**, whether **split vote** occurred (term bumps with **no** leader for multiple rounds — grep logs for `no leader` / failed elections).

**Theory link:** **randomized election timeout** spreads candidate start times so two nodes rarely become candidate in the same term window → avoids **split vote** livelock (see § Theory — Raft election).

---

## Step 3 — Partition 2 | 3 (minority vs majority)

**Goal:** Partition **{A,B}** vs **{C,D,E}** (2 minority, 3 majority). Only **majority** may **commit**; minority may **elect a false leader** locally but must **not commit** client writes that need global safety.

Use **iptables** on **Linux** (adjust interface — localhost often `lo`):

```bash
# Example: minority = node1,node2 (ports 8001,8002); majority = node3,4,5

# Drop ALL raft traffic between minority and majority on loopback
# (HashiCorp raft: peer TCP ports 8001-8005)

# Block 1,2 -> 3,4,5
for s in 8001 8002; do
  for d in 8003 8004 8005; do
    sudo iptables -A INPUT -i lo -p tcp --sport $s --dport $d -j DROP
    sudo iptables -A INPUT -i lo -p tcp --sport $d --dport $s -j DROP
  done
done

# Verify rules
sudo iptables -L INPUT -n -v | head -20
```

**While partitioned:**

```bash
# Writes via MAJORITY side leader (node on 9003..9005)
./client-load -leader "127.0.0.1:9003" -duration 5s -concurrency 2

# Writes via MINORITY side (expect timeout / not committed / leader unavailable)
./client-load -leader "127.0.0.1:9001" -duration 5s -concurrency 2 || true
```

**Record:** terms on each side, **commitIndex** on each node, client **OK vs fail**, whether minority logs **diverged** (same index, different term/entry).

**Heal partition:**

```bash
# Remove DROP rules (flush INPUT carefully on lab VM only)
sudo iptables -D INPUT ...   # repeat for each rule, or:
sudo iptables -F INPUT       # lab VM only — destructive

# Observe: follower sync, log truncation of uncommitted minority entries
grep -E 'truncate|compaction|AppendEntries|conflict' logs/node*.log
```

**Understanding check (deliverable):** draw **term / log index** diagram for partition + heal; label **truncated** entries and cite **log matching** + **commit rule** (§ Theory).

---

## Step 4 — Optional: `tc netem` (delay/loss, not partition)

```bash
# Add 200ms delay on lo (Linux) — slows elections/replication; partition still use iptables
sudo tc qdisc add dev lo root netem delay 200ms 50ms
# Remove: sudo tc qdisc del dev lo root
```

Use to see **replication lag** under delay; **CAP** intuition: higher latency without partition ≠ split brain.

---

## Step 5 — Single-node failure vs steady state

With **5 nodes**, stop **one non-leader** follower:

```bash
kill $(pgrep -f 'raft-node -id node4')
./client-load -leader "<current-leader-http>" -duration 10s
```

Compare **writes/sec** to Step 1 baseline (quorum **4/5** still OK). Then kill **second** follower (down to 3/5 — still quorum). At **3 failures** (2 survivors), **writes must fail** (no majority).

---

## Step 6 — Reads and consistency (lab policy)

For **linearizable** reads on the leader library path:

- **HashiCorp:** use **`raft.Apply`** for writes; for reads use **`VerifyLeader()`** or **`ReadIndex`**-style barrier (if implemented) before serving state.  
- **Never** serve arbitrary follower reads without **read quorum** unless you document **stale read** risk.

**Theory link:** committed log defines a **total order** of operations → **linearizable** state machine if all reads go through leader confirmation (§ Consistency).

---

## Step 7 — What to log for the diagram

During partition experiment, capture **per node**:

| Field | Use |
|-------|-----|
| `term` | election epochs |
| `role` | follower / candidate / leader |
| `lastLogIndex`, `lastLogTerm` | end of log |
| `commitIndex` | highest committed |
| each `Apply` | `(index, term, cmd)` |

After heal, diff minority vs majority logs at same index — **first conflicting index** → follower **truncates** suffix, then **appends** leader entries.

---

## Report template

1. **Election:** kill leader → `election_ms`, term change, split-vote yes/no  
2. **Partition 2|3:** commits on majority vs minority; terms on both sides  
3. **Heal:** which indices truncated on minority; final committed log identical on all survivors  
4. **Throughput:** steady 5-node vs 1 follower down vs during partition (majority only)  
5. **Diagram + paragraph:** safe truncation (Understanding check)

---

## Theory (curated for this lab)

### Why consensus is hard

Multiple nodes can **crash**, **delay**, or **partition**. Without agreement, two leaders could **commit conflicting writes** (**split brain**). **Consensus** picks **one** ordered log of commands so the state machine is **replicated** deterministically. **FLP:** no deterministic async consensus with one faulty process — Raft uses **timeouts**, **randomization**, and **majority quorums** for **liveness** in practice.

### Replication (§3.3 — leader-follower)

| Model | This lab |
|--------|----------|
| **Leader-follower** | **Raft** — only **leader** accepts client writes (proposals); followers replicate via **AppendEntries** |
| **Multi-leader / leaderless** | Not used — would complicate conflict resolution |
| **Sync vs async** | Commit to client after **majority persist** (semi-sync feel); followers may **lag** (**replication lag**) |
| **Quorum** | **Write quorum** = majority of **N** nodes ack append → entry **committed** |
| **Failover** | Leader dies → **election** → new leader continues log |
| **Split brain** | Prevented: old leader in minority cannot commit **new** entries in higher term on majority side |

### Raft (§3.4 — learn deeply)

**Leader election**

- Followers wait **election timeout** (random in `[min, max]`). On timeout → **candidate**, **term++**, vote for self, request votes.
- **Randomization** desynchronizes candidates → usually **one** wins per term → avoids **split vote** repeated ties.
- **Majority votes** → **leader** for that term.

**Log replication**

- Client cmd → **leader** appends to local log → **AppendEntries** to followers.
- Entry **committed** when **replicated on majority** *and* entry’s **term = leader’s current term** (commit rule).
- **Log matching:** if follower log diverges at index *i*, truncate follower suffix, copy leader prefix.

**Partition 2 | 3 (5 nodes)**

- **Majority (3):** can elect leader, **commit** new entries.
- **Minority (2):** may elect **local** leader in a **stale term**, but **cannot commit** entries that contradict majority’s committed history; client writes **must not** succeed if you enforce quorum/leader checks.
- **CAP:** under partition, Raft **chooses consistency** (linearizable history on majority) over **availability** on minority (**unavailable** for commits).

**Membership / failure detection**

- Lab uses fixed **5** peers; production adds **joint consensus** for membership changes (read Raft paper §6 — optional extension).

**Contrast (names only):** **Paxos / Multi-Paxos** — equivalent power, different decomposition; **Zab** (ZooKeeper); **BFT** — tolerates **malicious** nodes (not this lab).

### Time (§2 — what matters for Raft)

Raft correctness does **not** use **NTP/PTP** or **TrueTime**. It uses:

- **Logical order:** **term** (epoch), **log index**
- **Causality / happens-before:** if event *A* causes *B* (e.g. client write → replicate → commit → apply), **index order** captures it for clients that read via leader

**Physical clocks** only for **your metrics** (election latency). **Clock drift** does not define commit order.

Optional: **Lamport / vector clocks** appear in **causal consistency** systems; **Raft commit order** is **stronger** (total order on committed entries).

### Consistency (§3.5)

| Level | Raft committed reads (via leader) |
|--------|-----------------------------------|
| **Linearizability** | Yes, if reads use **leader** + **read index** / sync barrier |
| **Sequential / causal** | Weaker models; not goal of this lab |
| **Eventual** | Followers **without** read protocol may be stale |
| **Read-your-writes** | Client talks to same leader or tracks **commit index** |
| **CAP / PACELC** | Partition → **CP** (consistent, minority unavailable); **EL** without partition: latency vs replication |

### Distributed transactions (§3.6 — relation only)

**2PC / XA / sagas** coordinate **multiple services** with commit/abort. **Raft** replicates **one log** on **one service group** — often the **atomic broadcast** layer under a DB. Different problem; both avoid **partial commit** but at different layers.

---

## Understanding check — partition-and-heal diagram

Produce a diagram like:

```text
term 5  majority leader C:  [1:5][2:5][3:5][4:5] commit=4
term 4  minority "leader" A: [1:4][2:4][3:4']  (3' uncommitted, diverged)
        after heal A truncates index 3+, copies 3:5,4:5 from C
```

Explain in prose:

1. **Why** minority entry at index 3 was **uncommitted** (no majority ack in its term).  
2. **Why truncation** at first **term/index conflict** is **safe** (log matching; committed entries never overwritten on majority).  
3. **Why** clients on minority must **fail** (no writable quorum / stale leader).

---

## Pitfalls

| Issue | Fix |
|-------|-----|
| Two leaders after partition | Expected on minority **briefly**; verify **no commit** on minority for new client writes |
| `iptables` on macOS | Run lab on **GCP Ubuntu VM** (this doc) |
| All nodes candidate, no leader | Increase election timeout spread; check **port** connectivity |
| `go mod tidy`: max version 1.18 | **apt `golang-go` is too old** — install from [go.dev/dl](https://go.dev/dl/) (see Path A) |
| `missing go.sum entry` | **`git pull`** for committed `go.sum`, or run `go mod tidy` with Go **≥ 1.21** |
| Data dir reuse | `rm -rf data logs` between full replays; **`-bootstrap` only on node1** when data is empty |
| Commits on 2-node partition | Bug — check **Apply** only after **commitIndex** advance on **current term** |

---

## Quick reference

```bash
# Laptop → VM
gcloud compute ssh lab-raft-31 --zone=us-central1-a

# VM — build (Path A)
cd ~/systemlab/distributed/raft_lab_3_1 && go build -o raft-node ./cmd/raft-node

# VM — 5 nodes: node1 with -bootstrap once per fresh data/ (see Step 1)
PEERS="node1=127.0.0.1:8001,node2=127.0.0.1:8002,node3=127.0.0.1:8003,node4=127.0.0.1:8004,node5=127.0.0.1:8005"

# Partition minority 1,2 from 3,4,5 (iptables on lo — see Step 3)
# Heal: sudo iptables -F INPUT   # lab VM only

# Measure election: kill leader PID, poll /status for new leader + term
```

Source of truth for the lab is **`systemlab/distributed/raft_lab_3_1/`** in git; Path B is binary-only on `~/raft-lab/` if you skip the clone.
