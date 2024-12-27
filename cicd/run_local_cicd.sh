#!/bin/bash
set -e

cd sconnect/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplb/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplbmark/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplbdsr1/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplbdsr2/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplbl3dsr/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplbhash/
./config.sh
./validation.sh
./rmconfig.sh
cd -


cd sctplb/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd sctponearm/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd sctplbdsr/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplbmon/
./config.sh
./validation.sh
./rmconfig.sh
cd -
    
cd udplbmon/
./config.sh
./validation.sh
./rmconfig.sh
cd -
 
cd sctplbmon/
./config.sh
./validation.sh
./rmconfig.sh
cd -
  
cd tcplbmon6/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcplbepmod/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd lbtimeout/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd lb6timeout/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd httpsep/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd http2ep/
./config.sh
./validation.sh
./rmconfig.sh
cd -

cd tcpsctpperf
./config.sh
./validation.sh 20  30
./rmconfig.sh
cd -

cd tcpepscale/
./config.sh
./validation.sh
./rmconfig.sh
cd -
