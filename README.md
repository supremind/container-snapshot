# Container Snapshot Operator

Taking snapshots for docker containers running in kubernetes. 
It is rewritten base on [qiniu-ava/snapshot-operator](https://github.com/qiniu-ava/snapshot-operator), 
and is inspired by [wulibin163/kubepush](https://github.com/wulibin163/kubepush).

## How it works

1. the operator starts a worker, which:
2. runs `docker commit`, and
3. `docker push`

## How to use it

todo
