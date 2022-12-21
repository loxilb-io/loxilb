for i in `seq 0 $(($(getconf _NPROCESSORS_ONLN) - 1))`; do
    taskset -c $i ./wrk -t 1 -c 50 -d 60s -H 'Connection: close'  http://20.20.20.1:2020/ &
done
