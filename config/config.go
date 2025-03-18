// RaftConfig 对应 JSON 结构
package config
import (
	"encoding/json"
	"os"
)
type RaftConfig struct {
	ClusterName       string          `json:"cluster_name"`
	Nodes             []Node          `json:"nodes"`
	RaftDir           string          `json:"raft_dir"`
	ElectionTimeoutMs int             `json:"election_timeout_ms"`
	HeartbeatIntervalMs int           `json:"heartbeat_interval_ms"`
	Snapshot          SnapshotConfig  `json:"snapshot"`
	Log               LogConfig       `json:"log"`
	Transport         TransportConfig `json:"transport"`
	BootstrapExpect   int             `json:"bootstrap_expect"`
	SingleNode        bool            `json:"single_node"`
	Server            Server          `json:"server"`
}

type Node struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	RaftBind string `json:"raft_bind"`
}

type Server struct {
	Address string `json:"address"`
}

type SnapshotConfig struct {
	Enabled           bool `json:"enabled"`
	SnapshotIntervalSec int `json:"snapshot_interval_sec"`
	SnapshotThreshold int `json:"snapshot_threshold"`
	RetainSnapshots   int `json:"retain_snapshots"`
}

type LogConfig struct {
	LogDir        string `json:"log_dir"`
	TrailingLogs  int    `json:"trailing_logs"`
}

type TransportConfig struct {
	Type      string `json:"type"`
	MaxPool   int    `json:"max_pool"`
	TimeoutSec int    `json:"timeout_sec"`
}

func NewRaftConfig() *RaftConfig {
	return &RaftConfig{
		Snapshot: SnapshotConfig{
			Enabled:           false,
			SnapshotIntervalSec: 30,
			SnapshotThreshold: 1000,
			RetainSnapshots:   3,
		},
		Log: LogConfig{
			LogDir:        "/var/raft/logs",
			TrailingLogs:  10240,
		},
		Transport: TransportConfig{
			Type:      "tcp",
			MaxPool:   3,
			TimeoutSec: 5,
		},
	}
}
func LoadRaftConfig(path string) (*RaftConfig, error) {

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config RaftConfig
	if err := json.Unmarshal(b, &config); err != nil {	
		return nil, err
	}

	return &config, nil
}
