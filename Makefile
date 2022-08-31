nic = ip -o -4 route show to default | awk '{print $5}'

build: main.go network.go cgroup.go
	go build main.go network.go cgroup.go
	


bridge: 
	ip link add name veth1 type veth peer name br-veth1
	ip link add br0 type bridge
	ip link set dev br-veth1 master br0
	ip addr add 10.0.0.1/24 dev veth1
	ip link set veth1 up
	ip link set br-veth1 up
	ip link set br0 up
	iptables -t nat -A POSTROUTING -s 10.0.0.0/24 -o ens33 -j MASQUERADE

alpine:
	rm -r /newroot
	mkdir /newroot
	wget https://dl-cdn.alpinelinux.org/alpine/v3.16/releases/x86_64/alpine-minirootfs-3.16.2-x86_64.tar.gz
	tar -xzf alpine-minirootfs-3.16.2-x86_64.tar.gz -C /newroot
	rm alpine-minirootfs-3.16.2-x86_64.tar.gz

ubuntu:
	rm -r /newroot
	mkdir /newroot
	wget https://partner-images.canonical.com/oci/jammy/20220815/ubuntu-jammy-oci-amd64-root.tar.gz
	tar -xzf ubuntu-jammy-oci-amd64-root.tar.gz -C /newroot
	mkdir /newroot/newroot
	tar -xzf ubuntu-jammy-oci-amd64-root.tar.gz -C /newroot/newroot
	rm ubuntu-jammy-oci-amd64-root.tar.gz

simple:
	cp ../simple_container/simple /newroot/root