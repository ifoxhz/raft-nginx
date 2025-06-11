// Package RaftNode provides a simple distributed key-value RaftNode. The keys and
// associated values are changed via distributed consensus, meaning that the
// values are changed only when a majority of nodes in the cluster agree on
// the new value.
//
// Distributed consensus is provided via the Raft algorithm, specifically the
// Hashicorp implementation.
package raftnode

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
	"net/http"
	"bytes"
	"encoding/json"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/ifoxhz/raft-nginx/config"
	"github.com/ifoxhz/raft-nginx/helper"
)

var log = helper.Logger.Named("RaftNode")  // 创建子Logger

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)


// raftnode is raft wapper, where all changes are made via Raft consensus.
type RaftNode struct {
	RaftDir  string
	RaftBind string
	inmem    bool
	mu sync.Mutex
	raft *raft.Raft // The consensus mechanism
	fsm  *RaftFsm
	config config.RaftConfig
}

func New(f * RaftFsm) *RaftNode {
	return &RaftNode{
		fsm: f,
	}
}

// Open opens the RaftNode. If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (s *RaftNode) Open(enableSingle bool, localID string) error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)


	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.RaftBind)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(s.RaftBind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// Create the snapshot RaftNode. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(s.RaftDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot RaftNode: %s", err)
	}

	// Create the log RaftNode and stable RaftNode.
	var logStore raft.LogStore
	var stableStore raft.StableStore
	if s.inmem {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
	} else {
		log.Info("create RaftNode with ","RaftDir:", s.RaftDir)

		if err := os.MkdirAll(s.RaftDir, 0700); err != nil {
			log.Info("failed to create path for Raft storage: %s", err.Error())
			return err
		}

		boltDB, err := raftboltdb.New(raftboltdb.Options{
			Path: filepath.Join(s.RaftDir, "raft.db"),
		})
		if err != nil {
			return fmt.Errorf("new bbolt RaftNode: %s", err)
		}
		logStore = boltDB
		stableStore = boltDB
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, s.fsm, logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra

	if enableSingle {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		log.Info("raft enter bootstrap configuration: %v", configuration)
		ra.BootstrapCluster(configuration)
	}
	return nil
}

func (s *RaftNode) OpenWithcConfig(configInstance config.RaftConfig) error {
	
	s.config = configInstance
	s.RaftDir = configInstance.RaftDir
	s.RaftBind = configInstance.Nodes[0].RaftBind

	// Setup Raft configuration  from file config.json
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(configInstance.Nodes[0].ID)
	config.HeartbeatTimeout = time.Duration(configInstance.HeartbeatIntervalMs *1000*1000) // Convert ms to ns
	config.ElectionTimeout  = time.Duration(configInstance.ElectionTimeoutMs *1000*1000) // Convert ms to ns

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.RaftBind)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(s.RaftBind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Info("raft error NewTCPTransport: %v", s.RaftBind)
		return err
	}

	// Create the snapshot RaftNode. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(s.RaftDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot RaftNode: %s", err)
	}

	// Create the log RaftNode and stable RaftNode.
	var logStore raft.LogStore
	var stableStore raft.StableStore
	if s.inmem {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
	} else {
		log.Info("create RaftNode with RaftDir: %s", s.RaftDir)
		boltDB, err := raftboltdb.New(raftboltdb.Options{
			Path: filepath.Join(s.RaftDir, "raft.db"),
		})
		if err != nil {
			return fmt.Errorf("new bbolt RaftNode: %s", err)
		}
		logStore = boltDB
		stableStore = boltDB
	}

	// Instantiate the Raft systems.
	log.Info("init raft with config", "config", fmt.Sprintf("%+v", config))
	ra, err := raft.NewRaft(config, s.fsm, logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra

	if configInstance.SingleNode {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		log.Info("raft enter bootstrap configuration:", "config",configuration)
		ra.BootstrapCluster(configuration)
	}else{
		// configuration := raft.Configuration{
		// 	Servers: []raft.Server{
		// 		{
		// 			ID:      config.LocalID,
		// 			Address: raft.ServerAddress(configInstance.Server.Address),
		// 		},
		// 	},
		// }
		log.Info("raft enter join server: %v", configInstance.Server.Address)
		s.JoinCluster(configInstance.Server.Address, string(transport.LocalAddr()), string(config.LocalID))
	}

	return nil
}

// Join joins a node, identified by nodeID and located at addr, to this RaftNode.
// The node must be ready to respond to Raft communications at that address.
func (s *RaftNode) Join(nodeID, addr string) error {
	log.Info("received join request for remote node %s at %s", nodeID, addr)

	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Info("failed to get raft configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				log.Info("the","node",nodeID, "at" ,addr, "already member of cluster, ignoring join request")
				//write state file /dev/shm/raftstate
				filePath := "/dev/shm/raftstate"
				f, err := os.OpenFile(filePath, os.O_TRUNC|os.O_SYNC|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Info("Failed to open file ", filePath, err)
					return err
				}
				defer f.Close()
				raftState := struct {
					State string
					NodeAddr string
				}{
					State: s.raft.State().String(),
					NodeAddr: s.config.Nodes[0].Address,
				}
				raftJson, _ := json.Marshal(raftState)
		
				if _, err = f.WriteString(string(raftJson)); err != nil {
					log.Info("Failed to write to file %s: %v", filePath, err)
				}
				return nil
			}

			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeID, addr, err)
			}
		}
	}
	log.Info("raft AddNonvoter ", "nodeID",nodeID, "addr", addr)
	f := s.raft.AddNonvoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		log.Info("failed to add node %s at %s to cluster: %s", nodeID, addr, f.Error())
		return f.Error()
	}
	log.Info("node %s at %s joined successfully", nodeID, addr)
	return nil
}
func (s *RaftNode) GetRaftState( ) string {
	return s.raft.State().String()
}

// GetRaftNodeId returns the ID of the local Raft node.
func (s *RaftNode) GetRaftNodeLocalId() string {
	return s.config.Nodes[0].ID
}
func (s *RaftNode) GetRaft() *raft.Raft {
	log.Info("raftnode Raft","object", s.raft)
	return s.raft
}


func (s *RaftNode) Apply(l []byte) interface{} {
	return s.raft.Apply(l,raftTimeout)
}


func (s *RaftNode) JoinCluster(joinAddr, raftAddr, nodeID string) error {

	time.Sleep(5 * time.Second)
	b, err := json.Marshal(map[string]string{"addr": raftAddr, "id": nodeID})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application-type/json", bytes.NewReader(b))
	if err != nil {
		log.Info("failed to jion node %s at %s to cluster: %s", joinAddr, raftAddr,err)
		return err
	}else{
		log.Info("node %s at %s joined successfully,current raft state: %s", nodeID, joinAddr, s.raft.State().String())
		filePath := "/dev/shm/raftstate"
		f, err := os.OpenFile(filePath, os.O_TRUNC|os.O_SYNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Info("Failed to open file %s: %v", filePath, err)
			return err
		}
		defer f.Close()

		raftState := struct {
			State string
			NodeAddr string
		}{
			State: s.raft.State().String(),
			NodeAddr: s.config.Nodes[0].Address,
		}
		raftJson, _ := json.Marshal(raftState)

		if _, err = f.WriteString(string(raftJson)); err != nil {
			log.Info("Failed to write to file %s: %v", filePath, err)
		}
	}
	defer resp.Body.Close()
	return nil

}	
