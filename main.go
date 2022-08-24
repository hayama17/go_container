package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	cgroupsv2 "github.com/containerd/cgroups/v2"
)

func main() {
	switch os.Args[1] {
	case "run":
		parent()
	case "child":
		if err := child(); err != nil {
			panic(err)
		}
	default:
		panic("wat should I do")
	}
}

func parent() error {
	fmt.Println(fmt.Sprint(os.Getpid()))
	var netns string
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)

	//netns の指定があるかないか
	switch os.Args[2] {
	case "-n":
		cmd = exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[4:]...)...)
		netns = os.Args[3]
	default:
		netns = "go_container"
	}
	netnspath := "/var/run/netns/" + netns

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWUTS,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}

	//make netns
	log.Println("create netns")
	if _, err := os.Create(netnspath); err != nil {
		return fmt.Errorf(netns+"make failed: %w", err)
	}

	proc_path := "/proc/" + fmt.Sprint(cmd.Process.Pid) + "/ns/net"

	if err := syscall.Mount(proc_path, netnspath, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting /newroot: %w", err)
	}

	//set veth
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "host1",
			MTU:  1500},
		PeerName: "br-host1",
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("can not crate veth pair")
	}

	pair_index, err := netlink.VethPeerIndex(veth)
	if err != nil {
		return fmt.Errorf("can not get pair index")
	}
	fd, err := unix.Open(netnspath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("can not crate veth pair")
	}
	if err := netlink.LinkSetNsFd(veth, fd); err != nil {
		return fmt.Errorf("can not join veth")
	}

	pair_link, err := netlink.LinkByIndex(pair_index)
	if err != nil {
		return fmt.Errorf("can not get pair link by index")
	}

	bridge_link, err := netlink.LinkByName("br0")
	if err != nil {
		return fmt.Errorf("can not get bridge link by name ")
	}
	if err := netlink.LinkSetMaster(pair_link, bridge_link); err != nil {
		return fmt.Errorf("can not join veth")
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("ERROR", err)
		log.Println("delete netns")
		if err := syscall.Unmount(netnspath, syscall.MNT_DETACH); err != nil {
			return fmt.Errorf("unmount old root dir %w", err)
		}
		if err := os.Remove(netnspath); err != nil {
			return fmt.Errorf("rm %s: %w", netnspath, err)
		}
		os.Exit(1)
	}

	//delete netns
	log.Println("delete netns")
	if err := syscall.Unmount(netnspath, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root dir %w", err)
	}
	if err := os.Remove(netnspath); err != nil {
		return fmt.Errorf("rm %s: %w", netnspath, err)
	}

	return nil
}

func child() error {
	//set veth

	//set hostname
	log.Println("set hostname")
	if err := syscall.Sethostname([]byte("container")); err != nil {
		return fmt.Errorf("Setting hostname failed: %w", err)
	}

	log.Println("mount /proc")
	if err := syscall.Mount("proc", "/newroot/proc", "proc", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("Proc mount failed: %w", err)
	}

	//cgroup meory limit
	minMem := int64(1)                 // 1K
	maxMem := int64(500 * 1024 * 1024) //100M
	res := cgroupsv2.Resources{
		Memory: &cgroupsv2.Memory{
			Min: &minMem,
			Max: &maxMem,
		},
	}
	log.Println("create cgroup maneger")
	mgr, err := cgroupsv2.NewManager("/sys/fs/cgroup", "/go-container-cgroupv2", &res)
	if err != nil {
		return fmt.Errorf("creating cgroups v2: %w", err)
	}
	defer mgr.Delete()

	log.Println("register tasks to my-container")
	if err := ioutil.WriteFile("/sys/fs/cgroup/go-container-cgroupv2/cgroup.procs", []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("Cgroups register tasks to my-container namespace failed: %w", err)
	}

	//pivot root
	log.Println("prepare Rootfs")
	if err := syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("prepare Rootfs: %w", err)
	}

	log.Println("bind mount .//newroot")
	if err := syscall.Mount("/newroot", "/newroot", "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting /newroot: %w", err)
	}

	log.Println("mkdir /newroot/putold")
	if err := os.MkdirAll("/newroot/putold", 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	log.Println("pivot_root")
	if err := syscall.PivotRoot("/newroot", "/newroot/putold"); err != nil {
		return fmt.Errorf("pivot root: %w", err)
	}

	log.Println("cd /")
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("change dir to /: %w", err)
	}

	if err := syscall.Unmount("/putold", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root dir %w", err)
	}

	// setting network

	if lo, err := netlink.LinkByName("lo"); err != nil {
		return fmt.Errorf("unmount old root dir %w", err)
	} else {
		if err := netlink.LinkSetUp(lo); err != nil {
			return fmt.Errorf("lo set up: %w ", err)
		}
	}

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	return nil
}
