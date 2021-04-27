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

```
go run upscale.go -f model.yaml -kubeconfig /Users/avelichk/.kub
```

## Undeploy

To cleanup Kind cluster run:

```
make undeploy
```
