#!/bin/bash
source ../common.sh
echo SCENARIO-http2ep

servArr=( "server1" "server2" "server3" )
ep=( "31.31.31.1" "32.32.32.1" "33.33.33.1" )
code=0
function health() {
    if [ "$1" == "strict" ]; then
        opt="-strict"
        echo "HTTP2 with strict TLS probing" >&2
    else
        echo "HTTP2 with TLS probing" >&2
    fi

    $hexec l3ep1 ./server/server -host server1 -key 31.31.31.1/key.pem -cert 31.31.31.1/cert.pem -cacert minica.pem $opt > /dev/null 2>&1 &
    $hexec l3ep2 ./server/server -host server2 -key 32.32.32.1/key.pem -cert 32.32.32.1/cert.pem -cacert minica.pem $opt > /dev/null 2>&1 &
    $hexec l3ep3 ./server/server -host server3 -key 33.33.33.1/key.pem -cert 33.33.33.1/cert.pem -cacert minica.pem $opt > /dev/null 2>&1 &

    sleep 30
    code=0
    j=0
    waitCount=0
    while [ $j -le 2 ]
    do
        res=$($dexec llb1 curl -s --max-time 10 --cert  /opt/loxilb/cert/server.crt --key  /opt/loxilb/cert/server.key --cacert /opt/loxilb/cert/rootCA.crt https://${ep[j]}:8080/health)
        #echo $res >&2
        if [[ $res == "OK" ]]
        then
            echo "${servArr[j]} HTTPS $opt [$res]"  >&2
            j=$(( $j + 1 ))
        else
            echo "Waiting for ${servArr[j]}(${ep[j]})"  >&2
            waitCount=$(( $waitCount + 1 ))
            if [[ $waitCount == 10 ]];
            then
                echo "HTTPS connection with All Servers [NOK]"  >&2
                echo SCENARIO-httpsep [FAILED]  >&2
                exit 1
            fi
        fi
        sleep 1
    done

    for i in {1..4}
    do
    for j in {0..2}
    do
        #res=$($hexec l3h1 curl -s --key 10.10.10.1/key.pem --cert 10.10.10.1/cert.pem  --cacert minica.pem https://20.20.20.1:2020/1:2020)
        res=$($hexec l3h1 ./client/client -key 10.10.10.1/key.pem --cert 10.10.10.1/cert.pem  --cacert minica.pem -host 20.20.20.1:2020)
        echo $res  >&2
        exp="HTTP/2.0:${servArr[j]} HTTP/2.0:${servArr[j]} "
        if [[ $res != $exp ]]
        then
            echo "Expected : $exp, Received: $res" >&2
            code=1
        fi
        sleep 1
    done
    done
    $hexec l3ep1 killall -9 server > /dev/null 2>&1
    $hexec l3ep2 killall -9 server > /dev/null 2>&1
    $hexec l3ep3 killall -9 server > /dev/null 2>&1
    echo $code
}

code=$(health)
if [[ $code == 0 ]]
then
    echo SCENARIO-http2ep p1 [OK]
else
    echo SCENARIO-http2ep p1 [FAILED]
    exit $code
fi

sleep 2

code=$(health "strict")
if [[ $code == 0 ]]
then
    echo SCENARIO-http2ep p2 [OK]
else
    echo SCENARIO-http2ep p2 [FAILED]
    exit $code
fi

exit $code

