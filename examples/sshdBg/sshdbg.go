// Copyright 2012-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a basic init script.
package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	fp "path/filepath"
	"strings"
)

var (
	commands = []string{
		"/bbin/dhclient -ipv6=false -timeout 3600",
		"/bbin/ip a",
		"/bbin/ntpdate",
		"bg /bbin/sshd -port 22 -privatekey /root/.ssh/id_rsa -keys /root/.ssh/authorized_keys",
		"/bbin/elvish",
		"/bbin/shutdown halt",
	}
)

func main() {
	for _, line := range commands {
		log.Printf("Executing Command: %v", line)
		cmdSplit := strings.Split(line, " ")
		if len(cmdSplit) == 0 {
			continue
		}
		if cmdSplit[0] == "bg" {
			bg(cmdSplit[1:])
			continue
		}
		cmd := exec.Command(cmdSplit[0], cmdSplit[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			log.Print(err)
		}

	}
	log.Print("Uinit Done!")
}

func bg(args []string) {
	if len(args) == 0 {
		return
	}
	//bg_bbin_sshd
	pfx := "bg" + strings.ReplaceAll(args[0], "/", "_")
	tmpdir, err := ioutil.TempDir("", pfx)
	if err != nil {
		log.Print(err)
		return
	}
	stderr, err := os.Create(fp.Join(tmpdir, "stderr"))
	if err != nil {
		log.Print(err)
		return
	}
	stdout, err := os.Create(fp.Join(tmpdir, "stdout"))
	if err != nil {
		log.Print(err)
		return
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr, cmd.Stdout = stderr, stdout
	err = cmd.Start()
	if err != nil {
		log.Print(err)
		return
	}
	go func() {
		err = cmd.Wait()
		if err != nil {
			log.Printf("%s: %s", pfx, err)
		}
	}()
}
