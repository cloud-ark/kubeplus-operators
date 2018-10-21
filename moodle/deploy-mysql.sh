#!/bin/bash

echo "Deploying MySQL"

kubectl create -f artifacts/moodle-mysql.yaml

