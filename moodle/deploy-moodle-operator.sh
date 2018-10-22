#!/bin/bash

echo "Deploying Moodle Operator"

MINIKUBE_IP=`minikube ip`

echo "MINIKUBE IP:$MINIKUBE_IP"

rm -f artifacts/deploy-moodle-operator.yaml

sed "s/MINIKUBE_IP/$MINIKUBE_IP/g" artifacts/deploy-moodle-operator-minikube.yaml > artifacts/deploy-moodle-operator.yaml

kubectl create -f artifacts/deploy-moodle-operator.yaml

echo "Done."

echo "You can now create Moodle instances as follows:"
echo "kubectl apply -f artifacts/moodle1.yaml"
echo "kubectl describe moodles moodle1"
