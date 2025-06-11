package raftnode
import (
	"io"
	"sync"
	"time"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/ifoxhz/raft-nginx/helper"
	"github.com/ifoxhz/raft-nginx/store"
)

/*
	The FSM implements the Finite State Machine (FSM) interface
*/

type RaftFsm struct {
	mu    sync.RWMutex
	store *store.Store
	log  helper.Log
	RaftNodeId string 
}

func NewRaftFsm(s *store.Store) *RaftFsm {
	return &RaftFsm{
		mu:    sync.RWMutex{},
		store: s,
		log: helper.Logger.Named("RaftFsm"),
	}
}

/*
	Required by Raft FSM interface
*/
var idx = 0
func (rf *RaftFsm) Apply(l *raft.Log) interface{} {

	// This produces A LOT of logs
	rf.log.Debug("Received log", "index", l.Index, "data", string(l.Data))
	now := time.Now()
	// milliseconds := now.UnixNano() 
	tstr := fmt.Sprintf("FSM Applied 时间：%s", now.Format("2006-01-02 15:04:05.000"))
	value := fmt.Sprintf("%s: ApplyNode %s-%s",string(l.Data), rf.RaftNodeId, tstr)
	rf.log.Info("RaftFsm Applying", "index", l.Index, "value", value)
	return rf.store.FsmApply(l)
}

/*
	Required by Raft FSM interface
*/
func (rf *RaftFsm) Restore(r io.ReadCloser) error {
	
	rf.log.Debug("RaftFsm Restore")
	return rf.store.FsmRestore(r)
}

/*
	Required by Raft FSM interface
*/
func (rf *RaftFsm) Snapshot() (raft.FSMSnapshot, error) {
	/*
		Make sure that any future calls to f.Apply() don't change the snapshot.

		So basically doing a deep-copy here.
	*/
	rf.log.Debug("RaftFsm Snapshot")
	return rf.store.FsmSnapshot()
}


