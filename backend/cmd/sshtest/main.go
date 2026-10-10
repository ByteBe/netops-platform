package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
	"time"
)

func try(user, pass string) {
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
			ssh.KeyboardInteractive(func(_, _ string, q []string, e []bool) ([]string, error) {
				a := make([]string, len(q))
				for i := range a { a[i] = pass }
				return a, nil
			}),
		},
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{"ssh-rsa", "rsa-sha2-256", "rsa-sha2-512"},
		Timeout:           8 * time.Second,
		Config: ssh.Config{
			KeyExchanges: []string{"diffie-hellman-group1-sha1","diffie-hellman-group14-sha1","diffie-hellman-group14-sha256"},
			Ciphers:      []string{"aes128-cbc","aes128-ctr","aes256-ctr","3des-cbc"},
			MACs:         []string{"hmac-sha1","hmac-sha1-96"},
		},
	}
	c, err := ssh.Dial("tcp","10.16.192.254:22",cfg)
	if err != nil { fmt.Printf("%s/%s FAIL: %v\n", user, pass, err); return }
		s,err := c.NewSession()
		if err != nil { fmt.Println("SESSION FAIL:",err); return }
		out,err := s.Output("display bgp peer vpnv4")
		fmt.Printf("OUT(%d):\n%s\nERR:%v\n", len(out), out, err)
}

func main() {
	combos := [][2]string{
		{"GNXJDL","tssbs123.."},
		{"GNXJNY","tssbs123.."},
		{"GNXJDL","Tssbs@123.."},
		{"admin","admin"},
	}
	if len(os.Args) >= 3 { combos = [][2]string{{os.Args[1], os.Args[2]}} }
	for _, c := range combos { try(c[0],c[1]) }
}
