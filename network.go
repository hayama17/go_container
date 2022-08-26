package main

import (
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// netnsを作り、errorと実行netnsに繋がっているプロセス上のfd返す
func Make_netns(netns_path string, proc_path string) (int, error) {

	log.Println("create netns")
	if _, err := os.Create(netns_path); err != nil {
		return 0, err
	}

	if err := syscall.Mount(proc_path, netns_path, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return 0, err
	}

	fd, err := unix.Open(netns_path, unix.O_CLOEXEC, 0)

	if err != nil {
		return 0, err
	}
	return fd, nil

}

// 作りたいvethの名前とアタッチしたいnetnsに繋がってるfdとbridgeの名前を引数にしてveth pairを作成、片方を指定したnetnsに所属、もう片方をbrigeに所属させる
func Make_attach_veth(veth_name string, fd int, bridge_name string) error {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: veth_name,
			MTU:  1500},
		PeerName: "br-" + veth_name,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return err
	}
	pair_index, err := netlink.VethPeerIndex(veth)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetNsFd(veth, fd); err != nil {
		return err
	}

	pair_link, err := netlink.LinkByIndex(pair_index)
	if err != nil {
		return err
	}

	bridge_link, err := netlink.LinkByName(bridge_name)
	if err != nil {
		return err
	}
	if err := netlink.LinkSetMaster(pair_link, bridge_link); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(pair_link); err != nil {
		return err
	}

	return nil
}

// bridgeと繋がってるnic名(eth)と設定したいipアドレス(ip_address)を引数にとってnicにip_addressをセットそてloとnicをupにする
func Network_setup(eth string, ip_address string) error {
	if lo, err := netlink.LinkByName("lo"); err != nil {
		return fmt.Errorf("search link by lo %w", err)
	} else {
		if err := netlink.LinkSetUp(lo); err != nil {
			return fmt.Errorf("lo set up: %w ", err)
		}
	}
	if veth, err := netlink.LinkByName(eth); err != nil {
		return fmt.Errorf("search link by host1 %w", err)
	} else {
		addr, _ := netlink.ParseAddr(ip_address)
		netlink.AddrAdd(veth, addr)
		if err := netlink.LinkSetUp(veth); err != nil {
			return fmt.Errorf("lo set up: %w ", err)
		}
	}

	return nil
}
