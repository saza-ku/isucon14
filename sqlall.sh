set -eux

MYSQL_USER=<PLACEHOLDER_MYSQL_USER>
MYSQL_PASS=<PLACEHOLDER_MYSQL_PASS>
MYSQL_DBNAME=<PLACEHOLDER_MYSQL_DBNAME>

cd `dirname $0`

rsync -a ./scripts/exec.sql isucon@isucon1:~/scripts/exec.sql &
rsync -a ./scripts/exec.sql isucon@isucon2:~/scripts/exec.sql &
rsync -a ./scripts/exec.sql isucon@isucon3:~/scripts/exec.sql &
wait

ssh isucon@isucon1 "mysql -u$MYSQL_USER -p$MYSQL_PASS $MYSQL_DBNAME < ~/scripts/exec.sql" &
ssh isucon@isucon2 "mysql -u$MYSQL_USER -p$MYSQL_PASS $MYSQL_DBNAME < ~/scripts/exec.sql" &
ssh isucon@isucon3 "mysql -u$MYSQL_USER -p$MYSQL_PASS $MYSQL_DBNAME < ~/scripts/exec.sql" &
wait
