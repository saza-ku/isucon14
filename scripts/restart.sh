#!/bin/bash
set -eux

sudo systemctl restart nginx
sudo systemctl restart mysql
sudo systemctl restart {{ app_service_name }}
sudo test -f /var/log/nginx/access.log && sudo sh -c 'echo -n "" > /var/log/nginx/access.log'
sudo test -f /var/log/mysql/slow.log && sudo sh -c 'echo -n "" > /var/log/mysql/slow.log'
