================
Moodle Operator
================

Deployment on Minikube
-----------------------

Demo_

.. _Demo: https://drive.google.com/file/d/1KAMk131mOD_UQXmxzOQ1j_Aqle2sW49M/view


Try:
-----

0) Setup steps:

   - Install Golang and set GOPATH environment variable to the folder where you
     will store Go code.

   - Add Path to Golang binary (which go / whereis go) to your PATH environment variable.

   - Install Go's dep dependency management tool: https://github.com/golang/dep

   - Clone this repository and put it inside 'src' directory of your GOPATH at following location:

     $GOPATH/src/github.com/cloud-ark/kubeplus-operators

     - mkdir -p $GOPATH/src/github.com/cloud-ark

     - cd $GOPATH/src/github.com/cloud-ark/

     - git clone https://github.com/cloud-ark/kubeplus-operators.git

   - Install dependencies

     - cd  $GOPATH/src/github.com/cloud-ark/kubeplus-operators/moodle

     - dep ensure

   - Start Minikube

     - ./start-minikube.sh

1) Build Moodle Operator Image 

   - ./build-local-deploy-artifacts.sh

2) Deploy MySQL

   - ./deploy-mysql.sh

   - kubectl get pods

Once MySQL Pod is running, 

3) Deploy Moodle Operator

   - ./deploy-moodle-operator.sh

   - kubectl get pods

Once moodle-operator Pod is running,

4) Create Moodle Instance: Moodle1

   - kubectl apply -f artifacts/moodle1.yaml

   - As part of creating moodle instance, we install the 'profilecohort' plugin.
     Check the custom resource specification in artifacts/moodle1.yaml

5) Check Moodle Deployment

   - kubectl get pods
   - kubectl describe moodles moodle1 (Initial Moodle deployment will take some time)

   - Once moodle1 instance is Ready, navigate to the moodle1 instance URL in the browser

     - Login using "admin" and "password1" as credentials

     - Once logged in you will see a message to update Moodle database for 'profilecohort' plugin

       - Select that option to complete Plugin installation

     - Navigate to -> Administration -> Plugins -> Plugins Overview

     - You should see 'profilecohort' plugin in the 'Additional plugins' list

6) Update Moodle Deployment to install new Plugin

   - We will install 'wiris' plugin on 'moodle1' Moodle instance

   - kubectl apply -f artifacts/update-moodle1.yaml

   - Once moodle1 instance is Ready, refresh the URL

       - You will see a message to update Moodle database for 'wiris' plugin

       - Select that option to complete Plugin installation

     - Navigate to -> Administration -> Plugins -> Plugins Overview

     - You should see 'profilecohort' and 'wiris' plugins in the 'Additional plugins' list



