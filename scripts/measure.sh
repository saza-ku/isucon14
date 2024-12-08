#!/bin/bash
set -eux

cd `dirname $0`

MYSQL_USER=isucon
MYSQL_PASS=isucon
MYSQL_DBNAME=isuride
GITHUB_TOKEN=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
GITHUB_REPO=saza-ku/isucon14

# Clear logs
sudo test -f /var/log/nginx/access.log && sudo sh -c 'echo -n "" > /var/log/nginx/access.log'
sudo test -f /var/log/mysql/slow.log && sudo sh -c 'echo -n "" > /var/log/mysql/slow.log'

FROM=`date +%s%N | cut -b1-13`
DATE=`date +"%m%d%H%M"`

{% if main_server %}
./github/github create-issue --token $GITHUB_TOKEN --repo $GITHUB_REPO --date $DATE
{% else %}
sleep 5
{% endif %}

# pprof
mkdir -p ~/results/pprof
curl -o ~/results/pprof/$DATE http://localhost:6060/debug/pprof/profile?seconds=70 &
curl -o ~/results/pprof/fg-$DATE http://localhost:6060/debug/fgprof?seconds=70 &

sleep 75

TO=`date +%s%N | cut -b1-13`

# Measure
mkdir -p ~/results/$DATE

# Slow query log
sudo mysqldumpslow -s t /var/log/mysql/slow.log | head -n 30 > ~/results/$DATE/mysql-slow-origin.log
cut -c -500 ~/results/$DATE/mysql-slow-origin.log > ~/results/$DATE/mysql-slow.log

# MySQL explain
sudo cat /var/log/mysql/slow.log | pt-query-digest --explain \
    u=$MYSQL_USER,p=$MYSQL_PASS,D=$MYSQL_DBNAME --limit 4 \
    > ~/results/$DATE/mysql-explain-origin.log
cut -c -500 ~/results/$DATE/mysql-explain-origin.log > ~/results/$DATE/mysql-explain.log

# nginx access log
sudo alp ltsv --config alp.yaml > ~/results/$DATE/alp.log

# netdata url
echo "http://localhost:{{ netdata_port }}/v2#after=$FROM;before=$TO;local--chartName-val=menu_systemd" > ~/results/$DATE/netdata.txt
# for v1
# echo "http://localhost:{{ netdata_port }}/#menu_services;after=$FROM;before=$TO" > ~/results/$DATE/netdata.txt

# jaeger url
echo "http://eren:16686/search?service=webapp&end=${TO}000&start=${FROM}000&limits=1000" > ~/results/$DATE/jaeger.txt

./github/github comment-issue --token $GITHUB_TOKEN --repo $GITHUB_REPO --date $DATE --server {{ ansible_host }} --netdata-port {{ netdata_port }}
