package main

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	cfg := &ssh.ClientConfig{
		User: "GNXJNY",
		Auth: []ssh.AuthMethod{
			ssh.Password("tssbs123.."),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{"ssh-rsa", "rsa-sha2-256", "rsa-sha2-512"},
		Timeout:         15 * time.Second,
		Config: ssh.Config{
			KeyExchanges: []string{"diffie-hellman-group1-sha1", "diffie-hellman-group14-sha1", "curve25519-sha256"},
			Ciphers:      []string{"aes128-cbc", "aes128-ctr", "aes256-ctr", "3des-cbc"},
			MACs:         []string{"hmac-sha1", "hmac-sha2-256"},
		},
	}
	fmt.Println("Connecting...")
	client, err := ssh.Dial("tcp", "10.16.192.238:22", cfg)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	defer client.Close()
	fmt.Println("Connected!")

	session, _ := client.NewSession()
	modes := ssh.TerminalModes{ssh.ECHO: 0, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	session.RequestPty("vt100", 24, 80, modes)
	out, err := session.Output("display version")
	fmt.Println("OUTPUT:", string(out), "ERR:", err)
}
