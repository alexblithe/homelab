#!/bin/bash

TARGET_NAMESPACE=${TARGET_NAMESPACE:-"default"}

function install_chart() {
  TARGET_CHART=$1
  TARGET_HOST=$2
  HOSTS_FILE="hosts/$TARGET_HOST.yml"
  echo "Installing $TARGET_CHART... to $TARGET_HOST using $HOSTS_FILE"
  helm install $TARGET_CHART charts/$TARGET_CHART -n $TARGET_NAMESPACE -f $HOSTS_FILE --create-namespace
}

if [ "$1" == "all" ]; then
  for chart in charts/*; do
    if [ -d "$chart" ]; then
      TARGET_CHART=$(basename $chart)
      install_chart $TARGET_CHART localhost
    fi
  done
else
  install_chart $1 localhost
fi