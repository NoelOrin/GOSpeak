package service

import (
	"bufio"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"
)

// TestSendMailStartTLS_RejectsWithoutStartTLS 确保非 465 端口不会降级成明文 SMTP。
func TestSendMailStartTLS_RejectsWithoutStartTLS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := fmt.Fprint(conn, "220 fake ESMTP\r\n"); err != nil {
			return
		}
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return
		}
		if !strings.HasPrefix(line, "EHLO") && !strings.HasPrefix(line, "HELO") {
			return
		}
		// 只声明 8BITMIME，不声明 STARTTLS。
		if _, err := fmt.Fprint(conn, "250-fake\r\n250 8BITMIME\r\n"); err != nil {
			return
		}
	}()

	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	addr := net.JoinHostPort(host, port)
	auth := smtp.PlainAuth("", "user", "pass", host)
	err = sendMailStartTLS(addr, host, auth, "from@example.com", []string{"to@example.com"}, []byte("Subject: test\r\n\r\nbody"))
	if err == nil {
		t.Fatal("expected error when SMTP server does not advertise STARTTLS")
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected STARTTLS error, got: %v", err)
	}
	<-done
}
