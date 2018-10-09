#!/bin/bash

curl -Lo minikube-0.28 https://storage.googleapis.com/minikube/releases/v0.28.1/minikube-linux-amd64

chmod +x minikube-0.28

./minikube-0.28 start --extra-config=apiserver.service-node-port-range=1-32000 --kubernetes-version=v1.11.0


