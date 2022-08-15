package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
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

func parent() {
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
}

func child() error {
	log.Println("prepareRoot")
	if err := syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("prepare RootFS: %w", err)
	}

	log.Println("mkdir newroot/putold")
	if err := os.MkdirAll("newroot/putold", 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	log.Println("bind mount to ./newroot")
	if err := syscall.Mount("newroot", "newroot", "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting newroot: %w", err)
	}

	log.Println("run pivot_root")
	if err := syscall.PivotRoot("newroot", "newroot/putold"); err != nil {
		return fmt.Errorf("pivot root: %w", err)
	}

	log.Println("===========================================")
	log.Println("chdir / and go inside pivot root jail. Now we are in created container!")
	log.Println("===========================================")
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("change dir to /: %w", err)
	}

	// TIPS: by unmounting, parent resource will be hidden from child process
	if err := syscall.Unmount("/putold", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %w", err)
	}

	// mouting /proc inside container
	log.Println("mount /proc")
	// NOTE: somehow mount /proc fails in lima & nerdctl with EPERM
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mounting new /proc in container: %w", err)
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
