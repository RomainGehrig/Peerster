#!/usr/bin/env bash

DEBUG="true"

UIPort=8080
gossipPort=2000

i=$1
name="node$i"
rtimer=10
gossipPort=$(($gossipPort+i))
peerPort=$((($gossipPort-1)%10+2000))
UIPort=$(($UIPort+i))
peer="127.0.0.1:$peerPort"
gossipAddr="127.0.0.1:$gossipPort"
echo "./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer -rtimer=$rtimer"
if [[ "$DEBUG" == "true" ]] ; then
		echo "$name running at UIPort $UIPort and gossipPort $gossipPort"
fi
./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer -rtimer=$rtimer
