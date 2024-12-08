# 計測自動化の準備

## 事前
- [X] ~~*GitHub のトークンを発行しセット*~~ [2024-12-08]
- [X] ~~*あらかじめ ssh-keygen -R isucon1 isucon2 isucon3 を叩いておく*~~ [2024-12-08]

## 開始後
- [X] ~~*ローカルの /etc/hosts を書き換える（書式に気を付ける）*~~ [2024-12-08]
- [X] ~~*ssh できることを確認して `make init` を叩く*~~ [2024-12-08]
- [ ] sed.go の `TODO: settings` となっているところを全て埋めて `make prepare` を叩く
- [ ] init したやつの書き換え
  - [ ] nginx.conf を編集し、ltsv で解析できるようにする（[このようにする](https://github.com/Nagarei/isucon11-qualify-test/commit/b7e8f2667677831490d8e5966251633c14944015)）
    ```
        log_format ltsv "time:$time_local"
                "\thost:$remote_addr"
                "\tforwardedfor:$http_x_forwarded_for"
                "\treq:$request"
                "\tstatus:$status"
                "\tmethod:$request_method"
                "\turi:$request_uri"
                "\tsize:$body_bytes_sent"
                "\treferer:$http_referer"
                "\tua:$http_user_agent"
                "\treqtime:$request_time"
                "\tcache:$upstream_http_x_cache"
                "\truntime:$upstream_http_x_runtime"
                "\tapptime:$upstream_response_time"
                "\tvhost:$host";

        access_log  /var/log/nginx/access.log  ltsv;
      ```
  - [ ] nginx の config を編集し pprof の出力ファイルを配信するように設定する（[このようにする](https://github.com/Saza-ku/private-isu-2023/commit/d0ec5125783192884a9d164754e1f602f4e1a4c9#diff-c5ef4126bf2c674cca13a602dde349b38c227406c17b884109ded03afea1152fR17-R19)）（ポート 80 から HTTP でアクセスできることを確認する）
    ```
      location ~ /pprof {
        root /home/isucon/results/;
        try_files $uri =404;
      }
    ```
  - [ ] MySQL のスローログの設定（[このようにする](https://github.com/Saza-ku/isucon11q/commit/4b51aa65ccc2fe2e7055ef15d4c058b01e7c15f3#diff-28ca88da6aa2437d8b374172e457b049f0af076e11da2f0f7e8400875b0c0f6eR64-R66)）
    ```
    slow_query_log_file    = /var/log/mysql/slow.log
    slow_query_log         = 1
    long_query_time        = 0
    ```
  - [ ] /etc/hosts に isucon1, isucon2, isucon3, eren を追加（[このようにする](https://github.com/saza-ku/isucon11q-2024/commit/f17751cb2feab558d51f0da46dc5058b9116935e)）
    ```
    192.168.0.11 isucon1
    192.168.0.12 isucon2
    192.168.0.13 isucon3
    27.133.154.192 eren
    ```
  - [ ] main関数内で`measure.PrepareMeasure`を呼び出す（[このようにする](https://github.com/saza-ku/isucon11q-2024/commit/83f4adf21a2dfea1b0d8901f5ffc403f7b2ca2fe#diff-871eb89e86e63e7eca84f0075cba1a75574a11341cd89d39c7891864d2b085b9R251)）
    ```
    	measure.PrepareMeasure(e)
    ```
  - [ ] /initializeで`measure.CallSetup`を呼び出す（[このようにする](https://github.com/saza-ku/isucon11q-2024/commit/babc2a253e526e5bd24b20784a58969291659ee2)）
    ```
    	measure.CallSetup(port)
    ```
      - 複数台構成を見越してリバプロではなく app サーバーに直接飛ばす
- [ ] OpenTelemtryの計装（一旦これをする前に `make fulldeploy` して計測を行なっておくのが良い）
  - [ ] `measure.NewIsuconDB`を使ってDBのコネクションを得る（[このようにする](https://github.com/saza-ku/isucon11q-2024/commit/babc2a253e526e5bd24b20784a58969291659ee2)）
  - [ ] `context.Context`を引き回して、DBアクセスや外部API呼び出しに追加する（[このようにする](https://github.com/saza-ku/isucon11q-2024/commit/d1c16d395488fb36ff2d4d7358b936955a5a4a4b)）
- [ ] `make fulldeploy` でデプロイする

> [!NOTE]
> もしアプリケーションがコンテナ化されていた場合は[webappがコンテナ化されているときにしなければならないこと](https://github.com/saza-ku/isucon-template/wiki/webapp%E3%81%8C%E3%82%B3%E3%83%B3%E3%83%86%E3%83%8A%E5%8C%96%E3%81%95%E3%82%8C%E3%81%A6%E3%81%84%E3%82%8B%E3%81%A8%E3%81%8D%E3%81%AB%E3%81%97%E3%81%AA%E3%81%91%E3%82%8C%E3%81%B0%E3%81%AA%E3%82%89%E3%81%AA%E3%81%84%E3%81%93%E3%81%A8)を見る。

# 最速で計測を行う方法

- `make enable-measure`
- ベンチマーク回す
- `make result`
