===========
Moodle CRD
===========

1) kubectl create -f artifacts/moodlecrd.yaml

2) go run *.go -kubeconfig=$HOME/.kube/config

3) kubectl create -f artifacts/moodle.yaml

4) kubectl describe moodle moodle1
