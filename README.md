
=======
# raft-nginx
raft-nginx 是利用hashicorp的raft 协议，实现一个边缘计算nginx网关的集群框架.
Leader nginx支持边缘节点的读写操作，Follower节点的仅支持读操作
里面内置了一个hraftd类似的kv-store演示相关功能，但是重新组织了代码结构，可以通过自己的FSM 实现来完成新的后端服务接入，比如一个rocksdb.

待完善的点：
1.  实现完整的snapshot功能，算法：记录最后的apply id 和term, 在新的replacited logs过来的时候，需要校验是否已经applied 的logs
2.  核心的问题是在有状态的数据关联关系里面，实现snapshot 不被logs重放问题所困扰，是关键所在；并且同时要保持不被新的leader replicated 所覆盖或者重写


参考了[hraftd](https://github.com/otoolep/hraftd)

_For background on this project check out this [blog post](http://www.philipotoole.com/building-a-distributed-key-value-store-using-raft/)._

_You should also check out the GopherCon2023 talk "Build Your Own Distributed System Using Go" ([video](https://www.youtube.com/watch?v=8XbxQ1Epi5w), [slides](https://www.philipotoole.com/gophercon2023)), which explains step-by-step how to use the Hashicorp Raft library._



## Reading and writing keys
The reference implementation is a very simple in-memory key-value store. You can set a key by sending a request to the HTTP bind address (which defaults to `localhost:11000`):
```bash
curl -XPOST localhost:8100/key -d '{"foo": "bar"}'
```

You can read the value for a key like so:
```bash
curl -XGET localhost:8100/key/foo
```

## Running raft-nginx
*Building hraftd requires Go 1.20 or later.*

Starting and running a hraftd cluster is easy. Download and build hraftd like so:
```bash
mkdir work # or any directory you like
cd work
export GOPATH=$PWD
git clone git@github.com:ifoxhz/raft-nginx.git
cd raft-nginx
go install
```

Run your first hraftd node like so:
```bash
docker build -t raft-nginx:1.0.0 .
docker-compose up -d
```

You can now set a key and read its value back:
```bash
curl -XPOST localhost:8100/key -d '{"user1": "batman"}'
curl -XGET localhost:8100/key/user1
```

### Follower node
This tells each new node to join the existing node. Once joined, each node now knows about the key:
```bash
curl -XGET localhost:8200/key/user1
curl -XGET localhost:8300/key/user1
```

Furthermore you can add a second key:
```bash
curl -XPOST localhost:8100/key -d '{"user2": "robin"}'
```

Confirm that the new key has been set like so:
```bash
curl -XGET localhost:8100/key/user2
curl -XGET localhost:8200/key/user2
curl -XGET localhost:8300/key/user2
```

### Leader-forwarding
可以通过nginx的原生能力，转发request到Leader节点，现在的nginx没做配置

## Production use of Raft
For a production-grade example of using Hashicorp's Raft implementation, to replicate a SQLite database, check out [rqlite](https://github.com/rqlite/rqlite).


