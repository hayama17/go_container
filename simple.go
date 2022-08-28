package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func simple() {
	switch os.Args[1] {
	case "run":
		simple_parent()
	case "child":
		if err := simple_child(); err != nil {
			panic(err)
		}
	default:
		panic("wat should I do")
	}
}

func simple_parent() error {
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWUTS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}

	return nil
}

func simple_child() error {

	//set hostname
	log.Println("set hostname")
	if err := syscall.Sethostname([]byte("container")); err != nil {
		return fmt.Errorf("Setting hostname failed: %w", err)
	}

	//mount /proc
	log.Println("mount /proc")
	if err := syscall.Mount("proc", "/newroot/proc", "proc", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("Proc mount failed: %w", err)
	}

	//pivot root
	log.Println("prepare Rootfs")
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("prepare Rootfs: %w", err)
	}

	log.Println("bind mount /newroot")
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

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	return nil
}
