#!/bin/bash

if [ $# -ne 2 ]; then
    echo "Usage: $0 <deploy-group-name> <github-commit-id>"
    echo "$0 OfferC5 xxx"
    exit
fi

group=$1
commit_id=$2

deploy_output=$(aws deploy create-deployment --application-name OfferServer --deployment-group ${group} --deployment-config-name CustomerConfigTwenty --github-location commitId=${commit_id},repository=cloudadrd/offer_server --description="submit by shell")
deployment_id=$(echo $deploy_output | grep deploymentId | awk -F'"' '{print $4}')

if test -z "${deployment_id}"; then
    echo "deploy error, output: ${deploy_output}"
else
    echo "call the follow command to see result of deployment"
    echo
    echo "   aws deploy get-deployment --deployment-id ${deployment_id}"
    echo
fi

