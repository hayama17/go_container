# go_container

GO言語で自作したコンテナランタイム


## 必要なもの
* ./newroot
    * rootファイルシステムが必要
* Linux
## Quick Start


```bash
go build main.go
mkdir newroot
cd newroot
wget https://dl-cdn.alpinelinux.org/alpine/v3.16/releases/x86_64/alpine-minirootfs-3.16.2-x86_64.tar.gz
tar -xzf alpine-minirootfs-3.16.2-x86_64.tar.gz
cd ../
sudo ./main run /bin/sh
```

