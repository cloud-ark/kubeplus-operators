#!/bin/bash


eval $(minikube docker-env)

export GOOS=linux; go build .
mv moodle ./artifacts/moodle
docker build -t moodle-operator:latest ./artifacts/
rm ./artifacts/moodle



