## ISUCON X

### /etc/hosts
```
<PLACEHOLDER_ISUCON1_IP> isucon1
<PLACEHOLDER_ISUCON2_IP> isucon2
<PLACEHOLDER_ISUCON3_IP> isucon3
```

### SSH forwarding for netdata

```sh
ssh -fNT -L 0.0.0.0:19991:127.0.0.1:19999 isucon@isucon1
ssh -fNT -L 0.0.0.0:19992:127.0.0.1:19999 isucon@isucon2
ssh -fNT -L 0.0.0.0:19993:127.0.0.1:19999 isucon@isucon3
```

### Notes
