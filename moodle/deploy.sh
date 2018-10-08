#!/bin/bash

echo "Deploying MySQL"

kubectl create -f artifacts/moodle-mysql.yaml

MINIKUBE_IP=`minikube ip`

echo "MINIKUBE IP:$MINIKUBE_IP"

rm -f artifacts/deploy-moodle-operator-minikube-modified.yaml

sed "s/MINIKUBE_IP/$MINIKUBE_IP/g" artifacts/deploy-moodle-operator-minikube.yaml > artifacts/deploy-moodle-operator-minikube-modified.yaml

echo "Waiting for MySQL Pod to start"

sleep 15

echo "Deploying Moodle Operator"

kubectl create -f artifacts/deploy-moodle-operator-minikube-modified.yaml

echo "Done."

echo "You can now create Moodle instances as follows:"
echo "kubectl apply -f artifacts/create-moodle1.yaml"
echo "kubectl describe moodles moodle1"
