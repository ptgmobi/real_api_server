#!/bin/bash

pushd /opt/offer_server

killall tguard
killall tworker

sleep 1

nohup bin/tguard /pdata1/log/offer/tworker.log > /pdata1/log/offer/guard.log 2>&1 &

sleep 1

popd
