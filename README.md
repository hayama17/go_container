# go_container

GO言語で自作したコンテナランタイム


## 必要なもの
* /newroot
    * コンテナ用のrootファイルシステムが必要
* Linux
## Quick Start
```bash
cd ~
git clone https://github.com/hayama17/go_container.git
make build
sudo make bridge 
sudo mkdir /newroot
sudo cd /newroot
sudo wget https://dl-cdn.alpinelinux.org/alpine/v3.16/releases/x86_64/alpine-minirootfs-3.16.2-x86_64.tar.gz
sudo tar -xzf alpine-minirootfs-3.16.2-x86_64.tar.gz
sudo rm alpine-minirootfs-3.16.2-x86_64.tar.gz
cd ~/go_container
sudo ./main run /bin/sh
```
## オプション
* -n netns
    * network名前空間の名前をnetnsにする
    * defaultは"go_contaier"
* -i ip_address
    * コンテナのnicに指定したip_addressを付ける
    * defaultは"10.0.0.2/24"

## 仕様
* bridgeの名前を"br0"とハードコーディングしている為、変更できない
* Rootfsも/配下でハードコーディングしている為、変更できない