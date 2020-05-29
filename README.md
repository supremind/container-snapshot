# Container Snapshot Operator

Taking snapshots for docker containers running in kubernetes. 
It is a rewritten of [qiniu-ava/snapshot-operator](https://github.com/qiniu-ava/snapshot-operator), 
and is inspired by [wulibin163/kubepush](https://github.com/wulibin163/kubepush).

## How it works

1. the operator starts a worker, which behaves as:
2. runs `docker commit`, and
3. `docker push`

## How to use it

1. create CRD: `kubectl apply -f ./deploy/crds/atom.supremind.com_containersnapshots_crd.yaml`
2. deploy operator: `kubectl apply -f ./deploy`
3. create a ContainerSnapshot CR: `kubectl apply -f ./deploy/crds/atom.supremind.com_v1alpha1_containersnapshot_cr.yaml`.
    you may change the podName and containerName to a running pod
4. check to see the worker pod: `kubectl get po -w`
