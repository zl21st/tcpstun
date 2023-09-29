package stun

import (
	"context"
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func Control(network, address string, c syscall.RawConn) (err error) {
	if err := c.Control(func(fd uintptr) {
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if err != nil {
			return
		}

		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		if err != nil {
			return
		}
	}); err != nil {
		return err
	}
	return err
}

func ListenTcp(ctx context.Context, laddr string) (net.Listener, error) {
	cfg := net.ListenConfig{
		Control: Control,
	}
	return cfg.Listen(ctx, "tcp", laddr)
}

func DialTcp(laddr string, raddr string, timeout int) (net.Conn, error) {
	ip, port, err := net.SplitHostPort(laddr)
	if err != nil {
		return nil, err
	}

	lip := net.ParseIP(ip)
	lport, err := net.LookupPort("tcp", port)
	if err != nil {
		return nil, err
	}

	d := net.Dialer{
		Control:   Control,
		LocalAddr: &net.TCPAddr{IP: lip, Port: lport},
		Timeout:   time.Second * time.Duration(timeout),
	}
	return d.Dial("tcp", raddr)
}

// test if a remote host is reachable
func IsReachable(laddr, raddr string, timeout int) bool {
	conn, err := DialTcp(laddr, raddr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func CompareNatType(type1, type2 string) string {
	if type1 == type2 {
		return type1
	}

	if type1 == NatType0 || type2 == NatType0 {
		return NatType0
	}

	if type1 == NatType1 || type2 == NatType1 {
		return NatType1
	}

	if type1 == NatType2 || type2 == NatType2 {
		return NatType2
	}

	if type1 == NatType3 || type2 == NatType3 {
		return NatType3
	}

	if type1 == NatType4 || type2 == NatType4 {
		return NatType4
	}

	return NatTypeBlocked
}

func GetOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String(), nil
}
