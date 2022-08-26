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
