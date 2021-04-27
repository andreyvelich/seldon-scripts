#!/bin/bash

# This script is used to deploy Kind cluster with Cert manager and Seldon Core.
set -e

# Verify that appropriate apps are installed and Docker is running.
if [[ ! $(which git) || ! $(which docker) || ! $(which kind) || ! $(which kubectl) || ! $(which kustomize) || ! $(which go) ]]; then
  echo "You must install git, docker, kind, kubectl, kustomize and go to deploy Seldon in Kind cluster"
  exit 1
fi

echo "Creating Kind cluster..."
kind create cluster

# Set context for kubectl
kubectl config use-context kind-kind
kubectl get nodes

echo "Cloning Seldon core repository..."
temp_dir=$(mktemp -d)
git clone -b v1.7.0-release "git@github.com:SeldonIO/seldon-core.git" ${temp_dir}

echo "Deploying Cert Manager..."
cd $temp_dir/operator
make install-cert-manager

echo "Deploying Seldon core..."
# Cert manager requires some time before we can install Seldon manifests.
while ! make deploy; do
  echo "Retrying to deploy Seldon resources"
  sleep 10
done

echo "Seldon core has been deployed"
kubectl get pods -n seldon-system
