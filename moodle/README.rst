================
Moodle Operator
================

Deployment on Minikube
-----------------------

Demo_

.. _Demo: https://drive.google.com/file/d/1KAMk131mOD_UQXmxzOQ1j_Aqle2sW49M/view


Try:
-----

Follow https://github.com/cloud-ark/kubeplus/blob/master/examples/moodle/steps.txt


Development:
------------

1. Modify/Update code
2. Set Go paths (./setpaths.sh)
3. Create Moodle Operator Docker image (./build-local-deploy-artifacts.sh)
4. Deploy Moodle Operator
   - kubectl create -f artifacts/deploy-moodle-operator-minikube.yaml
5. Deploy Moodle Instance
   - Create namespace
     - kubectl ns customer2
   - Create MySQL
     - kubectl create -f artifacts/moodle2-mysql.yaml
   - Wait for MySQL deployment to become ready
     - kubectl get pods -n customer2
   - Create Moodle instance
     - kubectl create -f artifacts/moodle2.yaml
   - Wait for Moodle instance to become ready
     - kubectl get pods -n customer2
6. Check Moodle
   - kubectl describe moodles moodle2 -n customer2
   - Follow the steps to update etcd given in
     - https://github.com/cloud-ark/kubeplus/blob/master/examples/moodle/steps.txt#L70




How it works:
--------------

Moodle Operator creates Moodle instances from a base image.
This image consists of Nginx configured with Moodle code.

The Operator supports creation of multiple Moodle instances.
Each Moodle instance is available on a separate port.
This is made possible through combination of Operator code and a moodle setup script in the base image.

Steps that are performed in the Operator code:
- Generate a port number for each Moodle instance 
- Inject that port as an environment variable (MOODLE_PORT) when starting Moodle container from the base image (Nginx+Moodle image).
- Inject HOST_NAME as an environment variable into the Moodle container. The HOST_NAME is formed
  by concatinating Ingress path created for that Moodle instance and the generated port.
- Define a Container PostStart hook command consisting of following two steps: a) execution of moodle setup script in the base image, b) Reload Nginx in the base image.

The base Nginx+Moodle base image contains a script that sets up a moodle instance. It does following actions:
- Updates nginx's default.conf to listen on $MOODLE_PORT (value of MOODLE_PORT is available as environment variable set by the Operator). Before default.conf is updated, value of $MOODLE_PORT will have been resolved.
- Execute Moodle install command by passing $HOST_NAME as a parameter (value of HOST_NAME is available as environment variable set by the Operator.)

Once the Moodle container is running, the PostStart action configures it with correct port number and hostname, and then reloads nginx. Configuring the hostname as mentioned above ensures that resources (pages) of a Moodle instance are created with correct links. Updating Nginx config to listen to the generated port and then reloading it ensures that Nginx for that Moodle instance is be able to receive traffic on the generated port.
