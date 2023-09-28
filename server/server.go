package main

import (
	"encoding/gob"
	"flag"
	"net"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/zl21st/tcpstun/common"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:   false,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
}

type Server struct {
	Host1   string
	Host2   string
	Port1   int
	Port2   int
	Timeout int
}

func (s *Server) Check() {
	if s.Host1 == "" {
		log.Fatal("Host1 is empty")
	}
	if s.Host2 == "" {
		log.Fatal("Host2 is empty")
	}
	if s.Port1 == 0 {
		log.Fatal("Port1 is empty")
	}
	if s.Port2 == 0 {
		log.Fatal("Port2 is empty")
	}
	if s.Timeout == 0 {
		log.Fatal("Timeout is empty")
	}
}

func (s *Server) processConnection(conn net.Conn) {
	log.Debugf("connection accepted from %s", conn.RemoteAddr())

	// read the request
	var req common.ClientRequest
	err := gob.NewDecoder(conn).Decode(&req)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("decode client request failed")
		conn.Close()
		return
	}
	log.WithFields(log.Fields{
		"request": req,
	}).Debug("decode client request success")

	if req.Type != common.RequestType1 && req.Type != common.RequestType2 {
		log.WithFields(log.Fields{
			"request": req,
		}).Error("invalid request type")
		conn.Close()
		return
	}

	// send the response
	var res common.ServerResponse
	res.ClientLocalHost = req.LocalHost
	res.ClientLocalPort = req.LocalPort
	res.ClientMappedHost = conn.RemoteAddr().(*net.TCPAddr).IP.String()
	res.ClientMappedPort = conn.RemoteAddr().(*net.TCPAddr).Port
	res.ServerHost1 = s.Host1
	res.ServerHost2 = s.Host2
	res.ServerPort1 = s.Port1
	res.ServerPort2 = s.Port2

	err = gob.NewEncoder(conn).Encode(res)
	if err != nil {
		log.WithFields(log.Fields{
			"response": res,
			"error":    err,
		}).Error("encode server response failed")
		conn.Close()
		return
	}
	log.WithFields(log.Fields{
		"response": res,
	}).Debug("encode server response success")
	conn.Close()

	if req.Type == common.RequestType1 || (res.ClientMappedHost == res.ClientLocalHost && res.ClientMappedPort == res.ClientLocalPort) {
		return
	}

	s.sendServerRequests(res.ClientMappedHost + ":" + strconv.Itoa(res.ClientMappedPort))
}

func (s *Server) sendServerRequest(laddr, raddr, natype string) error {
	conn, err := common.DialTcp(laddr, raddr, s.Timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	var req common.ServerRequest
	req.Type = natype
	err = gob.NewEncoder(conn).Encode(req)
	if err != nil {
		return err
	}

	// read the response
	var res common.ClienResponse
	err = gob.NewDecoder(conn).Decode(&res)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) sendServerRequests(raddr string) {
	// send full cone nat detection request
	go func() {
		err := s.sendServerRequest(s.Host2+":"+strconv.Itoa(s.Port2), raddr, common.NatType1)
		if err != nil {
			log.WithFields(log.Fields{
				"raddr": raddr,
				"error": err,
			}).Debug("send full cone request failed")
		} else {
			log.WithFields(log.Fields{
				"raddr": raddr,
			}).Info("send full cone request success")
		}
	}()

	// send restricted nat detection request
	go func() {
		err := s.sendServerRequest(s.Host1+":"+strconv.Itoa(s.Port2), raddr, common.NatType2)
		if err != nil {
			log.WithFields(log.Fields{
				"raddr": raddr,
				"error": err,
			}).Debug("send restricted nat request failed")
		} else {
			log.WithFields(log.Fields{
				"raddr": raddr,
			}).Info("send restricted nat request success")
		}
	}()

	// send restricted port nat detection request
	go func() {
		// FIXME: should use s.Host1, but server will always return "connect: cannot assign requested address" error
		err := s.sendServerRequest(s.Host2+":"+strconv.Itoa(s.Port1), raddr, common.NatType3)
		if err != nil {
			log.WithFields(log.Fields{
				"raddr": raddr,
				"error": err,
			}).Debug("send restricted port nat request failed")
		} else {
			log.WithFields(log.Fields{
				"raddr": raddr,
			}).Info("send restricted port nat request success")
		}
	}()
}

func (s *Server) Start() {
	ln, err := common.ListenTcp(s.Host1 + ":" + strconv.Itoa(s.Port1))
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	log.Infof("listening on %s:%d", s.Host1, s.Port1)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("accept connection failed")
		}

		go s.processConnection(conn)
	}
}

func (s *Server) Stop() {
}

func main() {
	srv := Server{}
	flag.StringVar(&srv.Host1, "h1", "", "the first ip address")
	flag.StringVar(&srv.Host2, "h2", "", "the second ip address")
	flag.IntVar(&srv.Port1, "p1", 0, "the first port")
	flag.IntVar(&srv.Port2, "p2", 0, "the second port")
	flag.IntVar(&srv.Timeout, "O", 3, "connection timeout, in seconds")
	debug := flag.Bool("d", false, "enable debug mode")
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	srv.Check()

	srv.Start()
}
