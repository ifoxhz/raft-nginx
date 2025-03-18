package helper

import (
	"github.com/hashicorp/go-hclog"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"io"
)

type Log  hclog.Logger
var Logger hclog.Logger

func init() {

	// 打开或创建 /var/log/raft.log 文件
	// file, err := os.OpenFile("/var/log/raft.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
	// 	os.Exit(1)
	// }

	// defer file.Close()
	// 配置日志轮转
	logRotate := &lumberjack.Logger{
		Filename:   "./raft.log",
		MaxSize:    100,   // 单个文件最大100MB
		MaxBackups: 3,     // 保留3个旧文件
		MaxAge:     30,    // 保留30天
		Compress:   true,  // 压缩旧日志
	}

	Logger = hclog.New(&hclog.LoggerOptions{
		Name:  "raft-node",
		Level: hclog.LevelFromString("DEBUG"),
		Output:  io.MultiWriter(logRotate, os.Stdout),
	})
}
