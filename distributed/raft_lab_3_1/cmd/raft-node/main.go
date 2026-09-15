// Minimal 5-node-ready Raft peer (HashiCorp raft). Lab 3.1.
//
// Each process is a full Raft replica: there is no separate coordinator.
// - raft-bind: TCP between peers (AppendEntries, RequestVote, heartbeats).
// - http: lab client API (/write, /read, /status); clients talk to one node;
//   writes must hit the leader; quorum replication happens inside raft.Apply.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// setCmd is the JSON payload stored in each committed Raft log entry.
type setCmd struct {
	Op  string `json:"op"`
	Key string `json:"key"`
	Val string `json:"val"`
}

// kvFSM is the replicated state machine: all nodes apply the same log in order.
type kvFSM struct {
	mu sync.RWMutex
	m  map[string]string
}

func newKVFSM() *kvFSM {
	return &kvFSM{m: make(map[string]string)}
}

// Apply runs on every replica when an entry is committed (leader replicates first).
func (f *kvFSM) Apply(l *raft.Log) interface{} {
	var c setCmd
	if err := json.Unmarshal(l.Data, &c); err != nil {
		return err
	}
	if c.Op != "set" {
		return fmt.Errorf("unknown op %q", c.Op)
	}
	f.mu.Lock()
	f.m[c.Key] = c.Val
	f.mu.Unlock()
	return nil
}

// Snapshot / Restore let Raft compact the log; in-memory map is serialized to disk.
func (f *kvFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	cp := make(map[string]string, len(f.m))
	for k, v := range f.m {
		cp[k] = v
	}
	return &kvSnap{data: cp}, nil
}

func (f *kvFSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var cp map[string]string
	if err := json.NewDecoder(rc).Decode(&cp); err != nil {
		return err
	}
	f.mu.Lock()
	f.m = cp
	f.mu.Unlock()
	return nil
}

type kvSnap struct {
	data map[string]string
}

func (s *kvSnap) Persist(sink raft.SnapshotSink) error {
	err := json.NewEncoder(sink).Encode(s.data)
	if err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *kvSnap) Release() {}

func statU64(st map[string]string, key string) uint64 {
	var v uint64
	if s, ok := st[key]; ok {
		_, _ = fmt.Sscanf(s, "%d", &v)
	}
	return v
}

// parsePeers turns "node1=host:port,node2=..." into Raft server IDs and addresses.
func parsePeers(s string) map[raft.ServerID]raft.ServerAddress {
	out := make(map[raft.ServerID]raft.ServerAddress)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, addr, ok := strings.Cut(part, "=")
		if !ok {
			log.Fatalf("bad peer %q, want id=host:port", part)
		}
		out[raft.ServerID(id)] = raft.ServerAddress(addr)
	}
	return out
}

func main() {
	id := flag.String("id", "node1", "Raft server ID")
	raftBind := flag.String("raft-bind", "127.0.0.1:8001", "Raft TCP listen address")
	httpAddr := flag.String("http", "127.0.0.1:9001", "HTTP listen address")
	peers := flag.String("peers", "", "comma-separated id=host:port for all nodes")
	dataDir := flag.String("data", "", "data directory (default ./data/<id>)")
	bootstrap := flag.Bool("bootstrap", false, "bootstrap cluster (run once on fresh cluster, e.g. node1)")
	flag.Parse()

	if *peers == "" {
		log.Fatal("-peers required, e.g. node1=127.0.0.1:8001,...,node5=127.0.0.1:8005")
	}
	peerMap := parsePeers(*peers)
	if *dataDir == "" {
		*dataDir = filepath.Join("data", *id)
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	logPath := filepath.Join(*dataDir, "raft.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)

	fsm := newKVFSM()

	// Durable Raft state: log entries, term/vote metadata, periodic FSM snapshots.
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(*dataDir, "logs.db"))
	if err != nil {
		log.Fatal(err)
	}
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(*dataDir, "stable.db"))
	if err != nil {
		log.Fatal(err)
	}
	snapStore, err := raft.NewFileSnapshotStore(*dataDir, 2, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}

	// Inter-peer transport only; HTTP clients never use this port.
	addr := *raftBind
	advertise, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	maxPool := 3
	timeout := 10 * time.Second
	transport, err := raft.NewTCPTransport(addr, advertise, maxPool, timeout, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}

	cfg := raft.DefaultConfig()
	cfg.LocalID = raft.ServerID(*id)
	cfg.Logger = logger
	cfg.LogLevel = "INFO"

	r, err := raft.NewRaft(cfg, fsm, logStore, stableStore, snapStore, transport)
	if err != nil {
		log.Fatal(err)
	}

	// One-time seed of cluster membership when data dirs are empty (not a running coordinator).
	hasState, err := raft.HasExistingState(logStore, stableStore, snapStore)
	if err != nil {
		log.Fatal(err)
	}
	if !hasState && *bootstrap {
		var servers []raft.Server
		for sid, saddr := range peerMap {
			servers = append(servers, raft.Server{ID: sid, Address: saddr})
		}
		if err := r.BootstrapCluster(raft.Configuration{Servers: servers}).Error(); err != nil {
			log.Fatalf("bootstrap: %v", err)
		}
		logger.Printf("bootstrapped cluster with %d servers", len(servers))
	}

	var rref raft.Raft = r
	mux := http.NewServeMux()

	// Client discovery: any node can report current leader (see leaderId / leaderAddr).
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		st := rref.Stats()
		stats, _ := rref.GetConfiguration()
		leaderAddr, leaderID := rref.LeaderWithID()
		lastIdx := uint64(rref.LastIndex())
		commitIdx := statU64(st, "commit_index")
		term := statU64(st, "term")
		role := "follower"
		switch rref.State() {
		case raft.Candidate:
			role = "candidate"
		case raft.Leader:
			role = "leader"
		}
		_ = stats
		out := map[string]interface{}{
			"id":           *id,
			"leader":       rref.State() == raft.Leader,
			"role":         role,
			"term":         term,
			"leaderId":     string(leaderID),
			"leaderAddr":   string(leaderAddr),
			"lastIndex":    lastIdx,
			"commitIndex":  commitIdx,
			"state":        rref.State().String(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	// Leader appends to log, replicates to followers, waits for quorum commit, then Apply runs on all nodes.
	mux.HandleFunc("/write", func(w http.ResponseWriter, req *http.Request) {
		if rref.State() != raft.Leader {
			http.Error(w, "not leader", http.StatusServiceUnavailable)
			return
		}
		key, val := req.URL.Query().Get("key"), req.URL.Query().Get("val")
		if key == "" {
			http.Error(w, "missing key", http.StatusBadRequest)
			return
		}
		cmd, _ := json.Marshal(setCmd{Op: "set", Key: key, Val: val})
		fut := rref.Apply(cmd, 5*time.Second)
		if err := fut.Error(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Lab simplification: read local FSM on leader only (no ReadIndex / follower reads).
	mux.HandleFunc("/read", func(w http.ResponseWriter, req *http.Request) {
		if rref.State() != raft.Leader {
			http.Error(w, "not leader (lab: read via leader only)", http.StatusServiceUnavailable)
			return
		}
		key := req.URL.Query().Get("key")
		fsm.mu.RLock()
		val := fsm.m[key]
		fsm.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"key": key, "val": val})
	})

	logger.Printf("http %s raft %s id=%s", *httpAddr, addr, *id)
	log.Fatal(http.ListenAndServe(*httpAddr, mux))
}
