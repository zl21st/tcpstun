package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	"github.com/zl21st/tcpstun/stun"
)

func main() {
	cln := stun.Client{}
	flag.StringVar(&cln.ServerHost, "H", "", "server host")
	flag.IntVar(&cln.ServerPort, "P", 3478, "server port")
	flag.IntVar(&cln.Timeout, "O", 3, "timeout")
	flag.StringVar(&cln.LocalAddr, "i", "", "local address, ip or ip:port")
	flag.BoolVar(&cln.Basic, "B", false, "basic mode, do not detect NAT type")
	flag.BoolVar(&cln.Debug, "D", false, "enable debug mode")
	flag.Parse()

	if err := cln.Init(); err != nil {
		log.Fatal(err)
	}
	cln.Run()
	cln.PrintResult()

}
