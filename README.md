# Scripts to test Seldon Core

## Deploy

To deploy Kind cluster with Seldon core run:

```
make deploy
```

Wait until Seldon controller manager will be running:

```
$ kubectl get pods -n seldon-system

NAME                                         READY   STATUS    RESTARTS   AGE
seldon-controller-manager-6884bd657d-mk5z5   1/1     Running   0          39s
```

## Run upscale script

Install required Go packages by running:

```
go mod download
```

To deploy model deployment and upscale replicas to 2 run,
where `model.yaml` is the Seldon Deployment YAML path:

```
go run upscale.go -f model.yaml
```

This script uses kubeconfig from `$HOME/.kube/config`. If you want to use different
config run:

```
go run upscale.go -f model.yaml -kubeconfig <path-to-kubeconfig>
```

## Undeploy

To cleanup Kind cluster run:

```
make undeploy
```
