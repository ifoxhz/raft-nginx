// Package httpd provides the HTTP server for accessing the distributed key-value store.
// It also provides the endpoint for other nodes to join an existing cluster.
package httpd

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"os"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/raft"
	"github.com/ifoxhz/raft-nginx/helper"
	"github.com/ifoxhz/raft-nginx/raftnode"
	"github.com/ifoxhz/raft-nginx/store"
)

var log = helper.Logger.Named("service")  // 创建子Logger

type command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}



// Service provides HTTP service.
type Service struct {
	addr string
	ln   net.Listener
	store *store.Store
	raft  *raftnode.RaftNode
	router *chi.Mux
}

// New returns an uninitialized HTTP service.
func New(addr string, store * store.Store ,raft *raftnode.RaftNode) *Service {
	return &Service{
		addr:  addr,
		store: store,
		raft:raft,
		router :  chi.NewRouter(),
	}
}

// Start starts the service.
func (s *Service) Start() error {
	
	server := http.Server{
		Handler: s.router,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln

	s.InitMulService()
	s.InitRaftObserver()
	// http.Handle("/", s.mux)
	log.Info("starting HTTP server at ", "router", s.router)
	go func() {
		err := server.Serve(s.ln)
		if err != nil {
			log.Error("HTTP serve: %s", err)
		}
	}()

	return nil
}

// Close closes the service.
func (s *Service) Close() {
	s.ln.Close()
	return
}

// mux is the HTTP request multiplexer.

func (s *Service) InitMulService() {
	s.router.Use(middleware.Logger)
	s.router.Get("/key/{key}", s.handleKeyRequest)
	s.router.Post("/key", s.handleKeyRequest)
	s.router.Post("/join", s.handleJoin)
	s.router.Get("/raft", s.handleRaftRequest)
}

func (s *Service) InitRaftObserver( ) {
	stateChangeCh := make(chan raft.Observation)
	seeState := func(o *raft.Observation) bool { _, ok := o.Data.(raft.RaftState); return ok }
	go func() {
		for obValue := range stateChangeCh {
			log.Info("raft state changed to", "state", obValue)
			
			str := fmt.Sprintf("%+v", obValue.Raft)

			var addr string
			parts := strings.Split(str, "at ")
			if len(parts) > 1 {
    			addr = strings.Split(parts[1], " ")[0]
    			//ipPort = strings.Split(addr, ":")
            }
			
			raftState := struct {
				State string
				NodeAddr string
			}{
				State: fmt.Sprintf("%v", obValue.Data),
				NodeAddr: addr,
			}
			raftJson, _ := json.Marshal(raftState)
			s.writeRaftState(string(raftJson))
		}
	}()

	log.Info("raft","object",s.raft)
	s.raft.GetRaft().RegisterObserver(raft.NewObserver(stateChangeCh, false, seeState))
}



func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) {
	m := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(m) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	remoteAddr, ok := m["addr"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	nodeID, ok := m["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.raft.Join(nodeID, remoteAddr); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Service) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
	getKey := func() string {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 3 {
			return ""
		}
		return parts[2]
	}

	switch r.Method {
	case "GET":
		k := getKey()
		if k == "" {
			w.WriteHeader(http.StatusBadRequest)
		}
		v, err := s.store.Get(k)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		b, err := json.Marshal(map[string]string{k: v})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		io.WriteString(w, string(b))

	case "POST":
		// Read the value from the POST body.
		log.Info("node at raft ","state", s.raft.GetRaft().State())
		if s.raft.GetRaft().State() != raft.Leader {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return 
		}
		m := map[string]string{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for k, v := range m {
			if err := s.Set(k, v); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

	case "DELETE":
		log.Info("node at raft ", "state", s.raft.GetRaft().State())
		if s.raft.GetRaft().State() != raft.Leader {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return 
		}
		k := getKey()
		if k == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := s.Delete(k); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	return
}

// Addr returns the address on which the Service is listening
func (s *Service) Addr() net.Addr {
	return s.ln.Addr()
}

// Get raft state
func (s *Service) handleRaftRequest(w http.ResponseWriter, r *http.Request) {
	reState := struct {
		State string
		Node  string
	}{
		State: s.raft.GetRaftState(),
		Node:  s.raft.GetRaftNodeLocalId(),
	}
	jsonData, _ := json.Marshal(reState)

	// 设置响应头为 JSON 类型
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func (s *Service) writeRaftState(content string) {
	filePath := "/dev/shm/raftstate"
	f, err := os.OpenFile(filePath, os.O_TRUNC|os.O_SYNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Info("Failed to open file ", filePath, err)
		return
	}
	defer f.Close()

	if _, err = f.WriteString(content); err != nil {
		log.Error("Failed to write to file %s: %v", filePath, err)
	}
}

var idx = 0
func (s *Service) Set(key, value string) error {
	
	c := &command{
		Op:    "set",
		Key:   key,
		Value: value,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	idx++
	f := s.raft.Apply(b)

	if idx % 3 == 0 {
		time.Sleep(1 * time.Second) // 延迟一秒
		log.Info("save snapshot log")
		rs :=s.raft.GetRaft().Snapshot()
		if rs.Error() != nil {
			log.Info("snapshot error %s", rs.Error())
		}

	}
	return f.(raft.ApplyFuture).Error()
}

func (s *Service) Delete(key string) error {
	if s.raft.GetRaft().State() != raft.Leader {
		return fmt.Errorf("not leader")
	}

	c := &command{
		Op:  "delete",
		Key: key,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b)
	return f.(raft.ApplyFuture).Error()
}
