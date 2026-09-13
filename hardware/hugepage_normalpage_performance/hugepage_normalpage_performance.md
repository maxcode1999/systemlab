# Lab A1 — Huge Pages vs 4K Pages: Allocation & Fault Cost

Compare three ways to back a **4 GiB** anonymous region on Linux:


| Mode        | Mechanism                | Tradeoff                                          |
| ----------- | ------------------------ | ------------------------------------------------- |
| **4K**      | `mmap`                   | Many minor faults; high TLB pressure              |
| **Hugetlb** | `mmap(..., MAP_HUGETLB)` | Upfront `nr_hugepages`; fewer faults/TLB entries  |
| **THP**     | `madvise(MADV_HUGEPAGE)` | Kernel promotes via `khugepaged`; no upfront pool |


**Metrics:** minor page faults, wall time to touch the region, **dTLB load miss rate** on a random-access pass.

**Tools:** `MAP_HUGETLB`, `MADV_HUGEPAGE`, `perf stat -e page-faults,dTLB-load-misses,dTLB-loads`

---



## GCP VM (ephemeral)


| Item | Choice                                                                   |
| ---- | ------------------------------------------------------------------------ |
| RAM  | **≥ 16 GiB** (~4 GiB mapping + ~4 GiB hugetlb pool). 8 GiB is too tight. |
| Type | `e2-standard-4` (cheap) or `n2-standard-4` (stable perf)                 |
| OS   | Ubuntu 22.04/24.04 x86_64 (not Container-Optimized OS)                   |


```bash
# Default GCP project for all following gcloud commands
gcloud config set project YOUR_PROJECT_ID

# Create lab VM (4 vCPU, 16 GiB) with Ubuntu 22.04 boot disk
gcloud compute instances create lab-a1-hugepages \
  --zone=us-central1-a \
  --machine-type=e2-standard-4 \
  --image-family=ubuntu-2204-lts \
  --image-project=ubuntu-os-cloud \
  --boot-disk-size=30GB

# SSH into the VM (uses gcloud-managed keys)
gcloud compute ssh lab-a1-hugepages --zone=us-central1-a

# Delete VM when lab is done (stops compute and disk billing for this instance)
gcloud compute instances delete lab-a1-hugepages --zone=us-central1-a --quiet
```

---



## Step 1 — Baseline

```bash
# Huge page size/count and total RAM
grep -E 'Huge|MemTotal' /proc/meminfo

# THP policy: always | madvise | never
cat /sys/kernel/mm/transparent_hugepage/enabled

# Check if hugetlbfs is already mounted (|| true avoids error if none)
mount | grep hugetlbfs || true

# List perf events for data TLB (install perf first — Step 4)
perf list | grep -i dtlb
```

Expect **2 MiB** hugetlb: `Hugepagesize: 2048 kB`. Need `dTLB-loads` / `dTLB-load-misses` on x86.

---



## Step 2 — Explicit hugetlb (`MAP_HUGETLB`)

4 GiB → **2048** pages at 2 MiB each.

```bash
# Flush dirty pages to disk, then drop reclaimable kernel caches (lab hygiene)
sudo sync && sudo sysctl -w vm.drop_caches=3

# Reserve 2048 × 2 MiB hugetlb pages from physical RAM
sudo sysctl -w vm.nr_hugepages=2048

# Confirm reservation succeeded
grep -E 'HugePages_Total|HugePages_Free|Hugepagesize' /proc/meminfo
```

If `HugePages_Total` is **0**: 16 GiB+ VM, retry after `drop_caches`, or debug with `vm.nr_hugepages=512` (1 GiB).

```bash
# Persist hugetlb count across reboot (optional)
echo 'vm.nr_hugepages=2048' | sudo tee /etc/sysctl.d/99-hugepages.conf

# Reload all sysctl.d snippets
sudo sysctl --system
```

Hugetlbfs (Ubuntu often has it):

```bash
# Mount point for hugetlb-backed files (if you use file-backed huge mappings)
sudo mkdir -p /mnt/huge

# Add fstab line only if not already present
grep -q hugetlbfs /etc/fstab || echo 'nodev /mnt/huge hugetlbfs defaults 0 0' | sudo tee -a /etc/fstab

# Mount hugetlbfs at /mnt/huge
sudo mount -t hugetlbfs nodev /mnt/huge
```

Reserve **before** `mmap(..., MAP_HUGETLB)` or mmap fails.

---



## Step 3 — THP

```bash
# Only promote to THP when the app uses madvise(MADV_HUGEPAGE)
echo madvise | sudo tee /sys/kernel/mm/transparent_hugepage/enabled

# Promote eligible anonymous mappings to THP without madvise (optional; lab default is madvise)
# echo always | sudo tee /sys/kernel/mm/transparent_hugepage/enabled

# Bytes of anonymous memory backed by transparent huge pages (check during/after THP run)
grep AnonHugePages /proc/meminfo
```

No `nr_hugepages`. Optional defrag policy:

```bash
# Read current THP defrag/compaction behavior
cat /sys/kernel/mm/transparent_hugepage/defrag
```

---



## Step 4 — perf

```bash
# Refresh package index
sudo apt update

# Install perf and build tools matched to this kernel
sudo apt install -y linux-tools-common linux-tools-generic \
  "linux-tools-$(uname -r)" build-essential bc

# Fallback if versioned linux-tools package is missing
# sudo apt install -y "linux-cloud-tools-$(uname -r)"

# Allow non-root perf event counting (lab VM)
sudo sysctl -w kernel.perf_event_paranoid=1

# Smoke test: count CPU cycles for a no-op command
perf stat -e cycles true
```

---



## Step 5 — CPU isolation (optional)

Guest-only; **4 vCPUs** example: housekeeping **0–1**, isolated **2–3**. Skip if `cpu/isolated` is empty (no `taskset`).

**Where:** file `/etc/default/grub`, line `GRUB_CMDLINE_LINUX="..."` (double quotes). Append inside the quotes; **keep** existing tokens (`console=ttyS0`, etc.). Do **not** edit `/boot/grub/grub.cfg` by hand.

```bash
# Open the config
sudo nano /etc/default/grub
```

Example **before** (GCP Ubuntu often looks like this):

```bash
GRUB_CMDLINE_LINUX="console=ttyS0,115200 panic=-1"
```

Example **after** (one line, space-separated):

```bash
GRUB_CMDLINE_LINUX="console=ttyS0,115200 panic=-1 isolcpus=2-3 nohz_full=2-3 rcu_nocbs=2-3 irqaffinity=0-1"
```


| Token             | Role                           |
| ----------------- | ------------------------------ |
| `isolcpus=2-3`    | normal tasks off 2–3           |
| `nohz_full=2-3`   | tickless when one task on 2–3  |
| `rcu_nocbs=2-3`   | RCU callbacks off 2–3          |
| `irqaffinity=0-1` | new device IRQs default to 0–1 |


```bash
# Apply and reboot
sudo update-grub
sudo reboot
```

```bash
# Verify after reboot
cat /proc/cmdline | tr ' ' '\n' | grep -E 'isolcpus|nohz_full|rcu_nocbs|irqaffinity'
cat /sys/devices/system/cpu/isolated
```

**IRQs** — move **virtio / PCI device** lines to **0–1** (`echo 0-1 | sudo tee /proc/irq/N/smp_affinity_list`). Ignore **IPI, LOC, Timer, PMU** (per-CPU). Ban **2–3** for irqbalance: `IRQBALANCE_BANNED_CPUS=c` in `/etc/default/irqbalance`, then `sudo systemctl restart irqbalance`. Optional bulk (ignore failures):

```bash
grep -E '^ [0-9]' /proc/interrupts | head -20
for f in /proc/irq/*/smp_affinity_list; do echo 0-1 | sudo tee "$f" >/dev/null 2>&1 || true; done
```

**Bench:** `taskset -c 2-3 ./bench ...` · check with `mpstat -P ALL 1 5` (`sysstat`).

---



## Step 6 — Build benchmark (`bench_hugepages.cpp`)

Source: `hardware/bench_hugepages.cpp`. Modes: **4k** (`mmap`), **hugetlb** (`MAP_HUGETLB`), **thp** (`madvise(MADV_HUGEPAGE)`).

```bash
# Go to lab sources
cd hardware

# Compile optimized bench (Linux x86_64 VM)
g++ -O2 -std=c++17 -Wall -Wextra -o bench bench_hugepages.cpp

# Quick smoke test (64 MiB, no hugetlb pool needed)
./bench --mode 4k --size 67108864 --all --iterations 1000000
```

Bench prints `rdtscp_cycles=` on stdout for **touch** and **random** (random loop timed with **rdtscp** on x86). Convert to seconds: `cycles / (cpu_MHz × 1e6)` from `/proc/cpuinfo`.

---



## Step 7 — Measure (perf + rdtscp)

Full size **4294967296** (4 GiB). Run **hugetlb** only after Step 2; set **MODE** to each of `4k`, `hugetlb`, `thp`. If Step 5 isolated CPUs exist, wrap with `taskset -c …`.

```bash
# Mode under test
MODE=4k

# Page faults while touching the mapping (perf wall time at bottom)
perf stat -e page-faults,minor-faults,major-faults,cycles,instructions \
  taskset -c 2-3 ./bench --mode "$MODE" --size 4294967296 --touch-only

# dTLB (L1) + STLB (L2) hit/miss + page walks on random (populate + random; C4+ PMU on GCP)
# dtlb_load_misses.stlb_hit = L1 miss, L2 STLB hit | mem_inst_retired.stlb_miss_loads = STLB true miss
perf stat -e dTLB-loads,dTLB-load-misses,dtlb_load_misses.stlb_hit,mem_inst_retired.stlb_miss_loads,dtlb_load_misses.walk_completed,dtlb_load_misses.walk_completed_2m_4m,dtlb_load_misses.walk_completed_4k,cycles,instructions \
  taskset -c 2-3 ./bench --mode "$MODE" --size 4294967296 --random-pass --iterations 50000000

# Optional: 1 GiB (512 huge pages) — STLB often holds working set → ~800 walks vs ~50M for 4k
# SIZE=1073741824
# perf stat -e dTLB-loads,dTLB-load-misses,dtlb_load_misses.stlb_hit,mem_inst_retired.stlb_miss_loads,dtlb_load_misses.walk_completed,dtlb_load_misses.walk_completed_2m_4m,dtlb_load_misses.walk_completed_4k \
#   taskset -c 3 ./bench --mode "$MODE" --size "$SIZE" --random-pass --iterations 50000000
```

**L1:** `dTLB-load-misses / dTLB-loads`. **L2 hit (L1 miss, STLB hit):** `dtlb_load_misses.stlb_hit`. **L2 true miss:** `mem_inst_retired.stlb_miss_loads`. **Walks:** `dtlb_load_misses.walk_completed` (**2m_4m** vs **4k**). If `stlb_hit` ≫ `dTLB-load-misses`, treat as multiplex noise; trust **walk_completed**. **Runtime:** perf elapsed, bench `rdtscp_cycles`, or:

```bash
# GNU time (not the shell builtin)
/usr/bin/time -f 'wall_sec=%e' ./bench --mode 4k --size 4294967296 --touch-only
```

Repeat the two `perf stat` blocks for `MODE=hugetlb` and `MODE=thp`.

---



## Report (per mode)

1. Minor/page faults on touch
2. Wall time on touch
3. dTLB loads, misses, **walk_completed** (+ **2m_4m** / **4k**) on random pass
4. Note: `AnonHugePages` (THP) or `HugePages_*` (hugetlb)

**Theory (short):** larger pages → fewer populate faults and fewer TLB entries for 4 GiB → lower dTLB miss rate on random access. Hugetlb = upfront reservation; THP = promote-on-touch.

---



## Pitfalls


| Issue                       | Fix                                                                         |
| --------------------------- | --------------------------------------------------------------------------- |
| `nr_hugepages` stays 0      | 16 GiB+ RAM, `drop_caches`, or 512 pages for debug                          |
| `MAP_HUGETLB` fails         | Reserve first; check `HugePages_Free`                                       |
| THP like 4K                 | `enabled` not `madvise`/`always`; check `AnonHugePages`                     |
| perf denied                 | `kernel.perf_event_paranoid=1` (lab VM)                                     |
| tools package missing       | `linux-cloud-tools-$(uname -r)`                                             |
| Arm T2A                     | Different PMU names; lab targets x86                                        |
| E2 VM, cycles not supported | Use C4 + PMU; software events only on E2                                    |
| hyphenated walk event fails | Use `dtlb_load_misses.walk_completed` (see `perf list --no-desc`, grep tlb) |
| Empty `cpu/isolated`        | Skip Step 5 or complete grub reboot                                         |
| IRQs on 2–3                 | `irqaffinity=0-1`, `IRQBALANCE_BANNED_CPUS=c`, virtio IRQ → `0-1`           |


---



## Quick reference

```bash
# Free reclaimable cache before hugetlb reservation
sudo sync && sudo sysctl -w vm.drop_caches=3

# Reserve 4 GiB of 2 MiB hugetlb pages
sudo sysctl -w vm.nr_hugepages=2048

# THP only when madvise(MADV_HUGEPAGE) is used
echo madvise | sudo tee /sys/kernel/mm/transparent_hugepage/enabled
# echo always | sudo tee /sys/kernel/mm/transparent_hugepage/enabled

# Allow perf without root-only lockdown
sudo sysctl -w kernel.perf_event_paranoid=1

# Isolation check (see Step 5)
cat /sys/devices/system/cpu/isolated

# Build bench
g++ -O2 -std=c++17 -Wall -Wextra -o bench bench_hugepages.cpp

# Measure (MODE=4k | hugetlb | thp)
perf stat -e page-faults,minor-faults,major-faults \
  ./bench --mode "$MODE" --size 4294967296 --touch-only
perf stat -e dTLB-loads,dTLB-load-misses,dtlb_load_misses.stlb_hit,mem_inst_retired.stlb_miss_loads,dtlb_load_misses.walk_completed,dtlb_load_misses.walk_completed_2m_4m,dtlb_load_misses.walk_completed_4k \
  ./bench --mode "$MODE" --size 4294967296 --random-pass --iterations 50000000
```

