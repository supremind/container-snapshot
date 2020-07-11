[![Actions Status](https://github.com/supremind/container-snapshot/workflows/Container%20Snapshot/badge.svg)](https://github.com/supremind/container-snapshot/actions?query=workflow%3A%22Container+Snapshot%22)
[![Coverage Status](https://coveralls.io/repos/github/supremind/container-snapshot/badge.svg?branch=master)](https://coveralls.io/github/supremind/container-snapshot?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/supremind/container-snapshot)](https://goreportcard.com/report/github.com/supremind/container-snapshot) 


# Container Snapshot Operator

Taking snapshots for Docker containers running in Kubernetes.

This is a rewritten of [qiniu-ava/snapshot-operator](https://github.com/qiniu-ava/snapshot-operator), 
and is inspired by [wulibin163/kubepush](https://github.com/wulibin163/kubepush).


## How it works

1. The operator starts a worker. To communicate to the docker daemon runs the target container, the worker is configured to run on the same node of the target container.
2. The worker behaves as running `docker commit` to take a snapshot (as a new docker image) for the target container, and
3. running `docker push` to push the snapshot image.


## How to use it

1. preparation:

    1. create CRD:

            kubectl apply -f ./deploy/crds/atom.supremind.com_containersnapshots_crd.yaml

    1. deploy operator:

            kubectl apply -f ./deploy

2. use a ContainerSnapshot

    1. create a pod and make sure it is running:

            kubectl apply -f example/pod.yaml

    2. create an image push secret

        The easies way to do this is [creating one based on existing Docker credentials](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials), more options could be found in the same page.

    3. create a ContainerSnapshot CR:

            kubectl apply -f example/containersnapshot.yaml

        you may need to change the `imagePushSecrets.name` to your secret name, and the `image` to a repository you have write access to

3. check to see the worker pod starts and ends:

        kubectl get po -w

1. validate the generated snapshot image contains target container's read/write layer:

        docker run --rm my-snapshots/example-snapshot:v0.0.1 -- cat /dates


## Road map

- [ ] set worker pod template when start the operator
- [ ] set worker pod template in containerSnapshot spec
