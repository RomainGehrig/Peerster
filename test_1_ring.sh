#!/usr/bin/env bash

go build
cd client
go build
cd ..

RED='\033[0;31m'
NC='\033[0m'
DEBUG="true"

outputFiles=()
message=Weather_is_clear
message2=Winter_is_coming


UIPort=12345
gossipPort=5000
name='A'

# General peerster (gossiper) command
#./Peerster -UIPort=12345 -gossipPort=127.0.0.1:5001 -name=A -peers=127.0.0.1:5002 > A.out &

for i in `seq 1 10`;
do
	outFileName="$name.out"
	peerPort=$((($gossipPort+1)%10+5000))
	peer="127.0.0.1:$peerPort"
	gossipAddr="127.0.0.1:$gossipPort"
	./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -simple -peers=$peer > $outFileName &
	outputFiles+=("$outFileName")
	if [[ "$DEBUG" == "true" ]] ; then
		echo "$name running at UIPort $UIPort and gossipPort $gossipPort with peers=$peer and output file $outFileName"
	fi
	UIPort=$(($UIPort+1))
	gossipPort=$(($gossipPort+1))
	name=$(echo "$name" | tr "A-Y" "B-Z")
done

./client/client -UIPort=12349 -msg=$message
./client/client -UIPort=12346 -msg=$message2
sleep 5
pkill -f Peerster


#testing
failed="F"

if !(grep -q "CLIENT MESSAGE $message" "E.out") ; then
	echo "Failed bc of $message in E.out"
	failed="T"
fi

if !(grep -q "CLIENT MESSAGE $message2" "B.out") ; then
	echo "Failed bc of $message2 in B.out"
  failed="T"
fi

if [[ "$failed" == "T" ]] ; then
	echo -e "${RED}FAILED${NC}"
fi

# echo "${outputFiles[@]}"

gossipPort=5000
for i in `seq 0 9`;
do
	relayPort=$(($gossipPort-1))
	if [[ "$relayPort" == 4999 ]] ; then
		relayPort=5009
	fi
	nextPort=$((($gossipPort+1)%10+5000))
	msgLine="SIMPLE MESSAGE origin E from 127.0.0.1:$relayPort contents $message"
	msgLine2="SIMPLE MESSAGE origin B from 127.0.0.1:$relayPort contents $message2"
	peersLine="127.0.0.1:$nextPort,127.0.0.1:$relayPort"
	if [[ "$DEBUG" == "true" ]] ; then
		echo "check 1 $msgLine"
		echo "check 2 $msgLine2"
		echo "check 3 $peersLine"
	fi
	gossipPort=$(($gossipPort+1))
	if !(grep -q "$msgLine" "${outputFiles[$i]}") ; then
		echo "Failed bc of $messageLine for client ${outputFiles[$i]}"
   		failed="T"
	fi
	if !(grep -q "$peersLine" "${outputFiles[$i]}") ; then
		echo "Failed bc of $peerLine for client ${outputFiles[$i]}"
		failed="T"
	fi
	if !(grep -q "$msgLine2" "${outputFiles[$i]}") ; then
		echo "Failed bc of $msgLine2 for client ${outputFiles[$i]}"
		failed="T"
	fi
done

if [[ "$failed" == "T" ]] ; then
    echo -e "${RED}***FAILED***${NC}"
else
	echo "***PASSED***"
fi



#sleep 2
