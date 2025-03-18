#!/bin/sh

# 定义进程路径（根据实际路径调整）
OPENRESTY_BIN="/usr/local/openresty/bin/openresty"
SERVICE_BIN="//usr/local/bin/hraftd"  # 第二个进程的路径

# 进程启动函数
start_process() {
    echo "Starting: $@"
    "$@" &  # 后台运行进程
    local pid=$!
    echo "$pid" >> /var/run/service.pids  # 记录PID
    wait "$pid"  # 等待进程结束
    return $?
}

# 捕获终止信号（Docker stop 时触发）
trap 'kill -TERM $(cat /var/run/service.pids) 2>/dev/null' TERM INT QUIT

# 清理旧PID文件
rm -f /var/run/service.pids

# 并行启动两个进程（后台运行）
start_process $SERVICE_BIN -config /data/config.json &
start_process $OPENRESTY_BIN -g "daemon off;" &

# 等待所有后台进程结束
wait

# 清理并退出
rm -f /var/run/service.pids
exit $?
