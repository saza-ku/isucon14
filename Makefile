.PHONY: help
help:
	@echo "make deploy, d     : deploy webapp"
	@echo "make fulldeploy, f : deploy webapp and other settings files like etc/"
	@echo "make build, b      : build webapp"
	@echo "make build-github  : build github"
	@echo "make init, i       : set up the servers and fetch source codes and settings files"
	@echo "make prepare, p    : prepare the codes and settings files for the contest"
	@echo "make log, l        : fetch logs from the servers"

.PHONY: deploy
deploy: build
	rsync -a webapp/go/isuride isucon@isucon1:/home/isucon/webapp/go/isuride & \
	rsync -a webapp/go/isuride isucon@isucon2:/home/isucon/webapp/go/isuride & \
	rsync -a webapp/go/isuride isucon@isucon3:/home/isucon/webapp/go/isuride & \
	wait
	ssh isucon@isucon1 /home/isucon/scripts/restart.sh & \
	ssh isucon@isucon2 /home/isucon/scripts/restart.sh & \
	ssh isucon@isucon3 /home/isucon/scripts/restart.sh & \
	wait

.PHONY: fulldeploy
fulldeploy: build build-github
	cd ansible && ansible-playbook deploy.yml

.PHONY: build
build:
	cd webapp/go && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make

.PHONY: build-github
build-github:
	cd scripts/github && make

.PHONY: init
init:
	cd ansible && ansible-playbook init.yml

.PHONY: prepare
prepare:
	go run sed.go
	mv util webapp/go/util
	find webapp/go/util -type f ! -name "*.go" -delete
	cd webapp/go && go mod tidy
	cd scripts/github && make build

.PHONY: log
log:
	mkdir -p logs
	ssh isucon@isucon1 "sudo journalctl -u isuride-go.service -r | sed -n '/isucon-log-delimiter/q;p' | tac" > logs/isucon1.log
	ssh isucon@isucon2 "sudo journalctl -u isuride-go.service -r | sed -n '/isucon-log-delimiter/q;p' | tac" > logs/isucon2.log
	ssh isucon@isucon3 "sudo journalctl -u isuride-go.service -r | sed -n '/isucon-log-delimiter/q;p' | tac" > logs/isucon3.log

.PHONY: enable-measure
enable-measure:
	rsync -a scripts isucon@isucon1:/home/isucon/ & \
	rsync -a scripts isucon@isucon2:/home/isucon/ & \
	rsync -a scripts isucon@isucon3:/home/isucon/ & \
	wait
	./execall.sh "cat /home/isucon/scripts/enable-slow-log.sql | sudo mysql"
	test -f scripts/nginx.conf.backup || rsync -a isucon@isucon1:/etc/nginx/nginx.conf scripts/nginx.conf.backup
	./execall.sh "sudo cp /home/isucon/scripts/nginx.conf /etc/nginx/nginx.conf"
	./execall.sh "sudo systemctl restart nginx"

.PHONY: result
result:
	mkdir -p results/isucon1
	mkdir -p results/isucon2
	mkdir -p results/isucon3
	ssh isucon@isucon1 "sudo mysqldumpslow -s t /var/log/mysql/slow.log" > results/isucon1/mysql-slow.log & \
	ssh isucon@isucon2 "sudo mysqldumpslow -s t /var/log/mysql/slow.log" > results/isucon2/mysql-slow.log & \
	ssh isucon@isucon3 "sudo mysqldumpslow -s t /var/log/mysql/slow.log" > results/isucon3/mysql-slow.log & \
	wait
	ssh isucon@isucon1 "sudo alp ltsv --config /home/isucon/scripts/alp.yaml" > results/isucon1/alp.log & \
	ssh isucon@isucon2 "sudo alp ltsv --config /home/isucon/scripts/alp.yaml" > results/isucon2/alp.log & \
	ssh isucon@isucon3 "sudo alp ltsv --config /home/isucon/scripts/alp.yaml" > results/isucon3/alp.log & \
	wait
	./execall.sh "sudo test -f /var/log/nginx/access.log && sudo sh -c 'echo -n "" > /var/log/nginx/access.log'"
	./execall.sh "sudo test -f /var/log/mysql/slow.log && sudo sh -c 'echo -n "" > /var/log/mysql/slow.log'"

# alias
.PHONY: d
d: deploy

.PHONY: f
f: fulldeploy

.PHONY: b
b: build

.PHONY: i
i: init

.PHONY: p
p: prepare

.PHONY: l
l: log

.PHONY: r
r: result
