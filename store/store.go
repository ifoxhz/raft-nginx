package store
import (
	"sync"
	"fmt"
	"io"
	"encoding/json"
	// "time"
	"github.com/hashicorp/raft"
	"github.com/ifoxhz/raft-nginx/helper"
	// "github.com/syndtr/goleveldb/leveldb"
)

type Store struct {
	inmem    bool
	mu sync.Mutex
	m  map[string]string // The key-value store for the system.
	index uint64
	term  uint64
}


type command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}


func NewStore(inmem bool) *Store {
	return &Store{
		m:      make(map[string]string),
		inmem:  inmem,
	}
}


// Get returns the value for the given key.
func (st *Store) Get(key string) (string, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.m[key], nil
}



// Apply applies a Raft log entry to the key-value store.
func (st *Store) FsmApply(l *raft.Log) interface{} {
	var c command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}


	helper.Logger.Info("RaftFsm Apply set", "key", c.Key, "value", c.Value)

	st.index = l.Index
	st.term = l.Term

	switch c.Op {
	case "set":
		return st.applySet(c.Key, c.Value)
	case "delete":
		return st.applyDelete(c.Key)
	default:
		helper.Logger.Error(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
	return nil
}

// Snapshot returns a snapshot base realizing the store state.
func (st *Store) FsmSnapshot() (raft.FSMSnapshot, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	// Clone the map.
	o := make(map[string]string)
	for k, v := range st.m {
		o[k] = v
	}

	return &fsmSnapshot{store:o}, nil
}

// Restore stores the key-value store to a previous state.
func (st *Store) FsmRestore(rc io.ReadCloser) error {
	o := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}
	helper.Logger.Debug("store FsmRestore","json",o)
	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	st.m = o
	return nil
}

func (st *Store) applySet(key, value string) interface{} {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.m[key] = value
	return nil
}

func (st *Store) applyDelete(key string) interface{} {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.m, key)
	return nil
}

type fsmSnapshot struct {
	store map[string]string
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
	}

	return err
}

func (f *fsmSnapshot) Release() {}
