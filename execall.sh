set -eux

ssh isucon@isucon1 $@ &
ssh isucon@isucon2 $@ &
ssh isucon@isucon3 $@ &
wait
