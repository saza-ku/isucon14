# 使い方

[SETTINGS.md](SETTINGS.md) の通りに設定を行うと使えるようになる。

## Makefile
- make deploy
  - webapp のビルド
  - webapp を rsync でデプロイ
  - アプリケーションの変更だけをデプロイしたいときに使う
  - alias: `make d`
- make fulldeploy
  - webapp のビルド
  - Ansible の deploy.yml を叩く
  - ミドルウェアの設定なども含めて全てデプロイしたいときに使う
  - alias: `make f`
- make log
  - アプリケーションのログを取得しローカルの`logs/`に保存する
  - alias: `make l`
- make enable-slow-log
  - MySQL のスローログを有効にする
  - 最初の設定で手間取ったときに使う

## execall.sh

isucon1, isucon2, isucon3 の全てのサーバーに対し同じコマンドを実行する。

```bash
./execall.sh sudo ls -la /var/log/mysql
```

## sqlall.sh

isucon1, isucon2, isucon3 の全てのサーバーの MySQL に対し [scripts/exec.sql](../scripts/exec.sql) を実行する。  
インデックスを貼ったり初期データをいじったりといった手動オペレーション時に使う。

## Ansible

- init.yml
  - 基本的には試合開始時に一回叩く
  - 各サーバーで必要なパッケージをインストールする
  - サーバーから webapp や etc などのファイルを rsync でローカルに落としてくる
- deploy.yml
  - webapp, etc, scripts などを rsync でデプロイ
