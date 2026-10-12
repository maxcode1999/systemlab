# Conan ABI Lab — full instructions for Google Cloud

Chained packages (`libb` → `liba` → `app`), profile matrix, lockfiles, and
cross-glibc ABI on two GCE VMs. **Run Phase 8 (cleanup) when done.**

Work from your Mac inside **`conan-lab/`** (`cd` to this directory). All
`gcloud` commands below use:

```bash
export PROJECT_ID=project-d7462a20-f435-42a0-917
export ZONE=us-central1-a
gcloud config set project "$PROJECT_ID"
```

Run this **once per terminal** before any Phase. Commands use
`--project="$PROJECT_ID"` (not `$PROJECT`).

## Prerequisites

```bash
gcloud auth login
gcloud config set project "$PROJECT_ID"
gcloud services enable compute.googleapis.com --project="$PROJECT_ID"
```

---

## Phase 1 — Create the two VMs

```bash
gcloud compute instances create conan-old \
  --project="$PROJECT_ID" --zone="$ZONE" --machine-type=e2-small \
  --image-family=ubuntu-2204-lts --image-project=ubuntu-os-cloud \
  --preemptible

gcloud compute instances create conan-new \
  --project="$PROJECT_ID" --zone="$ZONE" --machine-type=e2-small \
  --image-family=ubuntu-2404-lts-amd64 --image-project=ubuntu-os-cloud \
  --preemptible

sleep 30
gcloud compute instances list --project="$PROJECT_ID" --filter="name~conan-"
```

- **conan-old** — Ubuntu 22.04, glibc ~2.35  
- **conan-new** — Ubuntu 24.04, glibc ~2.39  

(GCP removed `ubuntu-2004-lts`; 22.04 vs 24.04 still gives a real glibc gap.)

**Record:** both `RUNNING`.

---

## Phase 2 — Provision both VMs

Run for **conan-old** and **conan-new** (same block, change host name):

```bash
for host in conan-old conan-new; do
  echo "=== ${host} ==="
  gcloud compute ssh "$host" --project="$PROJECT_ID" --zone="$ZONE" --command='
    sudo apt-get update -y
    sudo apt-get install -y build-essential cmake python3-pip git \
      gcc-11 g++-11 clang-15 || true
    sudo apt-get install -y gcc-13 g++-13 clang-16 2>/dev/null || true
    pip3 install --user conan
    echo "export PATH=$HOME/.local/bin:$PATH" >> ~/.bashrc
    export PATH=$HOME/.local/bin:$PATH
    conan profile detect --force
    echo "---- glibc ----"
    ldd --version | head -1
    which gcc-11 gcc-13 clang-15 clang-16 2>/dev/null || true
  '
done
```

**Record:** both `ldd --version` lines.

---

## Phase 3 — Deploy the lab to both VMs

From **`conan-lab/`** on your Mac (parent of this file):

```bash
LAB_ROOT="$(pwd)"
for host in conan-old conan-new; do
  gcloud compute scp --recurse "$LAB_ROOT" "$host":~/conan-lab \
    --project="$PROJECT_ID" --zone="$ZONE"
done
```

---

## Phase 4 — Profile matrix + lockfiles (on conan-new)

### 4a — SSH in and run the matrix

```bash
gcloud compute ssh conan-new --project="$PROJECT_ID" --zone="$ZONE"
```

On the VM:

```bash
export PATH=$HOME/.local/bin:$PATH
cd ~/conan-lab

LIBB_VERSION="${LIBB_VERSION:-1.0}"
OUT=results.csv
echo "profile,libb_version,package_id,cache_status" > "$OUT"

sed -i "s/version = \".*\"/version = \"${LIBB_VERSION}\"/" libb/conanfile.py
conan export libb --version="${LIBB_VERSION}"

for profile in profiles/*.conf; do
  name=$(basename "$profile" .conf)
  echo "=== ${name} (libb ${LIBB_VERSION}) ==="
  sed -i "s#self.requires(\"libb/[0-9.]*\")#self.requires(\"libb/${LIBB_VERSION}\")#" liba/conanfile.py
  conan create libb --version="${LIBB_VERSION}" -pr:h="$profile" -pr:b="$profile" --build=missing > "/tmp/${name}.log" 2>&1 || true
  conan create liba -pr:h="$profile" -pr:b="$profile" --build=missing >> "/tmp/${name}.log" 2>&1 || true
  info=$(conan graph info app/conanfile.py -pr:h="$profile" -pr:b="$profile" --filter=binary 2>&1 || true)
  pkg_id=$(echo "$info" | grep -A2 "liba/1.0" | grep -m1 "ref:" | awk -F'#' '{print $2}')
  status=$(echo "$info" | grep -m1 "binary:" | awk '{print $2}')
  echo "${name},${LIBB_VERSION},${pkg_id:-unknown},${status:-unknown}" >> "$OUT"
done

column -s, -t "$OUT"
```

### 4b — Lockfile workflow (still on conan-new)

```bash
cd ~/conan-lab
export PATH=$HOME/.local/bin:$PATH
PROFILE=profiles/gcc13-release.conf
mkdir -p lockfiles

conan lock create app/conanfile.py -pr:h="$PROFILE" -pr:b="$PROFILE" \
  --lockfile-out=lockfiles/conan.lock
conan install app/conanfile.py -pr:h="$PROFILE" -pr:b="$PROFILE" \
  --lockfile=lockfiles/conan.lock --build=missing

sed -i 's/version = "1.0"/version = "2.0"/' libb/conanfile.py
conan export libb --version=2.0
sed -i 's#self.requires("libb/1.0")#self.requires("libb/2.0")#' liba/conanfile.py
conan create libb --version=2.0 -pr:h="$PROFILE" -pr:b="$PROFILE" --build=missing
conan create liba -pr:h="$PROFILE" -pr:b="$PROFILE" --build=missing
conan lock create app/conanfile.py -pr:h="$PROFILE" -pr:b="$PROFILE" \
  --lockfile-out=lockfiles/conan-bumped.lock
diff lockfiles/conan.lock lockfiles/conan-bumped.lock || true

sed -i 's#self.requires("libb/2.0")#self.requires("libb/1.0")#' liba/conanfile.py
sed -i 's/version = "2.0"/version = "1.0"/' libb/conanfile.py
exit
```

### 4c — Pull results to your Mac

```bash
mkdir -p results
gcloud compute scp conan-new:~/conan-lab/results.csv ./results/results.csv \
  --project="$PROJECT_ID" --zone="$ZONE"
gcloud compute scp conan-new:~/conan-lab/lockfiles/conan.lock ./results/conan.lock \
  --project="$PROJECT_ID" --zone="$ZONE"
gcloud compute scp conan-new:~/conan-lab/lockfiles/conan-bumped.lock ./results/conan-bumped.lock \
  --project="$PROJECT_ID" --zone="$ZONE"
```

**Record:** `results.csv`; `diff results/conan.lock results/conan-bumped.lock`.

---

## Phase 5 — Cross-glibc test

Build on **conan-new**, run on **conan-old** (expect `GLIBC_* not found`), then reverse.

```bash
gcloud compute ssh conan-new --project="$PROJECT_ID" --zone="$ZONE" --command='
  export PATH=$HOME/.local/bin:$PATH
  cd ~/conan-lab
  conan create app -pr:h=profiles/gcc13-release.conf -pr:b=profiles/gcc13-release.conf --build=missing
'

gcloud compute scp conan-new:'~/.conan2/p/b/app*/p/bin/app' ./app-from-new \
  --project="$PROJECT_ID" --zone="$ZONE" || echo "Run: conan cache path app/1.0 on conan-new"

gcloud compute scp ./app-from-new conan-old:~/app-from-new \
  --project="$PROJECT_ID" --zone="$ZONE"

gcloud compute ssh conan-old --project="$PROJECT_ID" --zone="$ZONE" --command='
  chmod +x ~/app-from-new && ~/app-from-new
' 2>&1 | tee cross_glibc_failure.log || true

gcloud compute ssh conan-old --project="$PROJECT_ID" --zone="$ZONE" --command='
  export PATH=$HOME/.local/bin:$PATH
  cd ~/conan-lab
  conan create app -pr:h=profiles/gcc11-release.conf -pr:b=profiles/gcc11-release.conf --build=missing
'

gcloud compute scp conan-old:'~/.conan2/p/b/app*/p/bin/app' ./app-from-old \
  --project="$PROJECT_ID" --zone="$ZONE" || true
gcloud compute scp ./app-from-old conan-new:~/app-from-old \
  --project="$PROJECT_ID" --zone="$ZONE"
gcloud compute ssh conan-new --project="$PROJECT_ID" --zone="$ZONE" --command='
  chmod +x ~/app-from-old && ~/app-from-old
'
```

**Record:** `cross_glibc_failure.log` and successful reverse run.

---

## Phase 6 — Symbol inspection

```bash
mkdir -p results
gcloud compute ssh conan-new --project="$PROJECT_ID" --zone="$ZONE" --command='
  objdump -T ~/conan-lab/app/build/Release/app 2>/dev/null | grep GLIBC | sort -u
' | tee results/symbols-new.txt

gcloud compute ssh conan-old --project="$PROJECT_ID" --zone="$ZONE" --command='
  objdump -T ~/conan-lab/app/build/Release/app | grep GLIBC | sort -u
' | tee results/symbols-old.txt

diff results/symbols-old.txt results/symbols-new.txt || true
```

---

## Phase 7 — Conan server on conan-old, pull from conan-new

```bash
gcloud compute ssh conan-old --project="$PROJECT_ID" --zone="$ZONE" --command='
  export PATH=$HOME/.local/bin:$PATH
  pip3 install --user conan_server
  nohup conan_server > server.log 2>&1 &
  sleep 3
  cat server.log
'

gcloud compute firewall-rules create allow-conan-server \
  --project="$PROJECT_ID" --allow=tcp:9300 --target-tags=http-server 2>/dev/null || true

OLD_IP=$(gcloud compute instances describe conan-old --project="$PROJECT_ID" --zone="$ZONE" \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)')
echo "conan-old IP: $OLD_IP"

gcloud compute ssh conan-old --project="$PROJECT_ID" --zone="$ZONE" --command="
  export PATH=\$HOME/.local/bin:\$PATH
  conan remote add local http://localhost:9300 2>/dev/null || true
  conan upload liba/1.0 -r=local --confirm
"

gcloud compute ssh conan-new --project="$PROJECT_ID" --zone="$ZONE" --command="
  export PATH=\$HOME/.local/bin:\$PATH
  conan remote add old-server http://${OLD_IP}:9300 2>/dev/null || true
  conan install --requires=liba/1.0 -r=old-server --build=never
" | tee results/remote_pull.log
```

---

## Phase 8 — Cleanup

```bash
gcloud compute instances delete conan-old conan-new \
  --project="$PROJECT_ID" --zone="$ZONE" --quiet
gcloud compute firewall-rules delete allow-conan-server \
  --project="$PROJECT_ID" --quiet 2>/dev/null || true
gcloud compute instances list --project="$PROJECT_ID"
```

---

## What to hand in

1. `results/results.csv`  
2. `results/conan.lock` and `results/conan-bumped.lock` + diff  
3. `cross_glibc_failure.log`  
4. `results/symbols-old.txt`, `results/symbols-new.txt`, diff  
5. Phase 7 paragraph (Conan settings vs glibc)  
6. Short explanation of GLIBC symbol versioning  
