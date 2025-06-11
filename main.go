package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	httpd "github.com/ifoxhz/raft-nginx/http"
	"github.com/ifoxhz/raft-nginx/raftnode"
	"github.com/ifoxhz/raft-nginx/config"
	"github.com/ifoxhz/raft-nginx/helper"
	"github.com/ifoxhz/raft-nginx/store"
)

var log = helper.Logger

// Command line defaults
const (
	DefaultHTTPAddr = "localhost:11000"
	DefaultRaftAddr = "localhost:12000"
)

// Command line parameters
var inmem bool
var httpAddr string
var raftAddr string
var joinAddr string
var nodeID string
var configFile string


func init() {
	flag.BoolVar(&inmem, "inmem", false, "Use in-memory storage for Raft")
	flag.StringVar(&httpAddr, "haddr", DefaultHTTPAddr, "Set the HTTP bind address")
	flag.StringVar(&raftAddr, "raddr", DefaultRaftAddr, "Set Raft bind address")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&nodeID, "id", "", "Node ID. If not set, same as Raft bind address")
	flag.StringVar(&configFile, "config", "", "Configuration file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <raft-data-path> \n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	
	var rfstore *store.Store 
	var fsm   *raftnode.RaftFsm
	var raftNode *raftnode.RaftNode
	

	if configFile != "" {
		config, err := config.LoadRaftConfig(configFile)
		if (err != nil) {
			log.Info("failed to open raft config: %s", err.Error())
			return 	
		}else{
			log.Info("Raft configuration loaded:", fmt.Sprintf("%+v", config)) 	
		}

		rfstore = store.NewStore(false)
		fsm   = raftnode.NewRaftFsm(rfstore)
		raftNode = raftnode.New(fsm)

		fsm.RaftNodeId = config.Nodes[0].ID
	

		log.Info("new store created ","rftstore", fsm.RaftNodeId)
		if err := raftNode.OpenWithcConfig(*config); err != nil {
			log.Error("failed to open store: %s", err.Error())
			os.Exit(-1)
		}
		if config.Nodes[0].Address != "" {
			httpAddr = config.Nodes[0].Address
		}

	}else{
			
		if nodeID == "" {
			nodeID = raftAddr
		}

		// Ensure Raft storage exists.
		raftDir := flag.Arg(0)
		if raftDir == "" {
			log.Error("No Raft storage directory specified")
			os.Exit(-2)
		}
		if err := os.MkdirAll(raftDir, 0700); err != nil {
			log.Error("failed to create path for Raft storage: %s", err.Error())
			os.Exit(-2)
		}

		rfstore = store.NewStore(inmem)
		fsm   = raftnode.NewRaftFsm(rfstore)
		raftNode = raftnode.New(fsm)

		fsm.RaftNodeId = nodeID
		log.Info("new store created ","rftstore", rfstore)
		log.Info("raft node id","nodeId", nodeID, "fsmRaftNodeId", fsm.RaftNodeId)

		raftNode.RaftDir = raftDir
		raftNode.RaftBind = raftAddr
		if err := raftNode.Open(joinAddr == "", nodeID); err != nil {
			log.Error("failed to open store: %s", err.Error())
			os.Exit(-2)
		}
	}


	h := httpd.New(httpAddr,rfstore,raftNode)
	if err := h.Start(); err != nil {
		log.Error("failed to start HTTP service: %s", err.Error())
		os.Exit(-2)
	}

	// If join was specified, make the join request.
	if joinAddr != "" {
		if err := join(joinAddr, raftAddr, nodeID); err != nil {
			log.Info("failed to join node at %s: %s", joinAddr, err.Error())
		}
	}

	// We're up and running!
	log.Info("hraftd started successfully, listening on http://%s", httpAddr)

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	log.Info("hraftd exiting")
}

func join(joinAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(map[string]string{"addr": raftAddr, "id": nodeID})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application-type/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
