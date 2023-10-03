package main

import (
	"flag"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/zl21st/tcpstun/stun"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:   false,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
}

func main() {
	srv := stun.Server{}
	flag.StringVar(&srv.Host1, "h1", "", "the first ip address")
	flag.StringVar(&srv.Host2, "h2", "", "the second ip address")
	flag.IntVar(&srv.Port1, "p1", 0, "the first port")
	flag.IntVar(&srv.Port2, "p2", 0, "the second port")
	flag.IntVar(&srv.Timeout, "O", 3, "connection timeout, in seconds")
	flag.BoolVar(&srv.Basic, "B", false, "basic mode, do not detect NAT type")
	debug := flag.Bool("D", false, "enable debug mode")
	version := flag.Bool("version", false, "show version")
	flag.Parse()

	if *version {
		fmt.Println(stun.Version)
		return
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	srv.Check()

	srv.Start()
}
