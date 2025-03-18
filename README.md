
=======
# raft-nginx
raft-nginx 是利用hashicorp的raft 协议，实现一个边缘计算nginx网关的集群框架.
Leader nginx支持边缘节点的读写操作，Follower节点的仅支持读操作
里面内置了一个hraftd类似的kv-store演示相关功能，但是重新组织了代码结构，可以通过自己的FSM 实现来完成新的后端服务接入，比如一个rocksdb.

待完善的点：

1.  Apply的log, 可能重新被写入后端服务；
2.  snapshot记录最后的log, 新的选举，不会重新写入已经apply的log到后端；


参考了[hraftd](https://github.com/otoolep/hraftd)

_For background on this project check out this [blog post](http://www.philipotoole.com/building-a-distributed-key-value-store-using-raft/)._

_You should also check out the GopherCon2023 talk "Build Your Own Distributed System Using Go" ([video](https://www.youtube.com/watch?v=8XbxQ1Epi5w), [slides](https://www.philipotoole.com/gophercon2023)), which explains step-by-step how to use the Hashicorp Raft library._



## Reading and writing keys
The reference implementation is a very simple in-memory key-value store. You can set a key by sending a request to the HTTP bind address (which defaults to `localhost:11000`):
```bash
curl -XPOST localhost:11000/key -d '{"foo": "bar"}'
```

You can read the value for a key like so:
```bash
curl -XGET localhost:11000/key/foo
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
```

You can now set a key and read its value back:
```bash
curl -XPOST localhost:11000/key -d '{"user1": "batman"}'
curl -XGET localhost:11000/key/user1
```

### Bring up a cluster
_A walkthrough of setting up a more realistic cluster is [here](https://github.com/otoolep/hraftd/blob/master/CLUSTERING.md)._

Let's bring up 2 more nodes, so we have a 3-node cluster. That way we can tolerate the failure of 1 node:
```bash
$GOPATH/bin/hraftd -id node1 -haddr localhost:11001 -raddr localhost:12001 -join :11000 ~/node1
$GOPATH/bin/hraftd -id node2 -haddr localhost:11002 -raddr localhost:12002 -join :11000 ~/node2
```
_This example shows each hraftd node running on the same host, so each node must listen on different ports. This would not be necessary if each node ran on a different host._

This tells each new node to join the existing node. Once joined, each node now knows about the key:
```bash
curl -XGET localhost:11000/key/user1
curl -XGET localhost:11001/key/user1
curl -XGET localhost:11002/key/user1
```

Furthermore you can add a second key:
```bash
curl -XPOST localhost:11000/key -d '{"user2": "robin"}'
```

Confirm that the new key has been set like so:
```bash
curl -XGET localhost:11000/key/user2
curl -XGET localhost:11001/key/user2
curl -XGET localhost:11002/key/user2
```

#### Stale reads
Because any node will answer a GET request, and nodes may "fall behind" updates, stale reads are possible. Again, hraftd is a simple program, for the purpose of demonstrating a distributed key-value store. If you are particularly interested in learning more about issue, you should check out [rqlite](https://rqlite.io/). rqlite allows the client to control [read consistency](https://rqlite.io/docs/api/read-consistency/), allowing the client to trade off read-responsiveness and correctness.

Read-consistency support could be ported to hraftd if necessary.

### Tolerating failure
Kill the leader process and watch one of the other nodes be elected leader. The keys are still available for query on the other nodes, and you can set keys on the new leader. Furthermore, when the first node is restarted, it will rejoin the cluster and learn about any updates that occurred while it was down.

A 3-node cluster can tolerate the failure of a single node, but a 5-node cluster can tolerate the failure of two nodes. But 5-node clusters require that the leader contact a larger number of nodes before any change e.g. setting a key's value, can be considered committed.

### Leader-forwarding
Automatically forwarding requests to set keys to the current leader is not implemented. The client must always send requests to change a key to the leader or an error will be returned.

## Production use of Raft
For a production-grade example of using Hashicorp's Raft implementation, to replicate a SQLite database, check out [rqlite](https://github.com/rqlite/rqlite).


