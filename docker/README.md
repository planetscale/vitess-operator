#local docker testing

build the image
```
./docker/docker.sh build
```

run the image
```
./docker/docker.sh run
```

once inside use scripts to start etcd and kube-apiserver
```
3ad722450f17:/vtop# docker/scripts/etcd-up.sh
starting etcd on http://127.0.0.1:2379
etcd is up
3ad722450f17:/vtop# docker/scripts/kube-apiserver-up.sh
starting kube-apiserver on https://127.0.0.1:5000
kube-apiserver is up
```
