## ISUCON X

### /etc/hosts
```
18.177.189.17 isucon1
52.194.49.98  isucon2
54.92.69.133  isucon3
```

### SSH forwarding for netdata

```sh
ssh -fNT -L 0.0.0.0:19991:127.0.0.1:19999 isucon@isucon1
ssh -fNT -L 0.0.0.0:19992:127.0.0.1:19999 isucon@isucon2
ssh -fNT -L 0.0.0.0:19993:127.0.0.1:19999 isucon@isucon3
```

### Notes
