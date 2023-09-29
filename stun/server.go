package stun

import (
	"context"
	"encoding/gob"
	"fmt"
	"net"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type Server struct {
	Host1   string
	Host2   string
	Port1   int
	Port2   int
	Timeout int
	Basic   bool
	cancel  context.CancelFunc
}

func (s *Server) Check() {
	if s.Host1 == "" {
		log.Fatal("Host1 is empty")
	}
	if s.Port1 == 0 {
		log.Fatal("Port1 is empty")
	}
	if s.Timeout == 0 {
		log.Fatal("Timeout is empty")
	}

	if !s.Basic {
		if s.Host2 == "" {
			log.Fatal("Host2 is empty")
		}
		if s.Port2 == 0 {
			log.Fatal("Port2 is empty")
		}
	}

}

func (s *Server) processConnection(conn net.Conn) {
	log.Debugf("connection accepted from %s", conn.RemoteAddr())

	// read the request
	var req ClientRequest
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

	if req.Type != RequestType1 && req.Type != RequestType2 {
		log.WithFields(log.Fields{
			"request": req,
		}).Error("invalid request type")
		conn.Close()
		return
	}

	// send the response
	var res ServerResponse
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
		"client": fmt.Sprintf("%s:%d=>%s:%d", res.ClientLocalHost, res.ClientLocalPort, res.ClientMappedHost, res.ClientMappedPort),
	}).Info("send server response success")
	conn.Close()

	if s.Basic || req.Type == RequestType1 || (res.ClientMappedHost == res.ClientLocalHost && res.ClientMappedPort == res.ClientLocalPort) {
		return
	}

	s.sendServerRequests(res.ClientMappedHost + ":" + strconv.Itoa(res.ClientMappedPort))
}

func (s *Server) sendServerRequest(laddr, raddr, natype string) error {
	conn, err := DialTcp(laddr, raddr, s.Timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	var req ServerRequest
	req.Type = natype
	err = gob.NewEncoder(conn).Encode(req)
	if err != nil {
		return err
	}

	// read the response
	var res ClienResponse
	err = gob.NewDecoder(conn).Decode(&res)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) sendServerRequests(raddr string) {
	// send full cone nat detection request
	go func() {
		laddr := s.Host2 + ":" + strconv.Itoa(s.Port2)
		err := s.sendServerRequest(laddr, raddr, NatType1)
		if err != nil {
			log.WithFields(log.Fields{
				"laddr": laddr,
				"raddr": raddr,
				"error": err,
			}).Debug("send full cone request failed")
		} else {
			log.WithFields(log.Fields{
				"laddr": laddr,
				"raddr": raddr,
			}).Info("send full cone request success")
		}
	}()

	// send restricted nat detection request
	go func() {
		laddr := s.Host1 + ":" + strconv.Itoa(s.Port2)
		err := s.sendServerRequest(laddr, raddr, NatType2)
		if err != nil {
			log.WithFields(log.Fields{
				"laddr": laddr,
				"raddr": raddr,
				"error": err,
			}).Debug("send restricted nat request failed")
		} else {
			log.WithFields(log.Fields{
				"laddr": laddr,
				"raddr": raddr,
			}).Info("send restricted nat request success")
		}
	}()

	// send restricted port nat detection request
	go func() {
		// FIXME: should use s.Host1, but server will always return "connect: cannot assign requested address" error
		laddr := s.Host2 + ":" + strconv.Itoa(s.Port1)
		err := s.sendServerRequest(laddr, raddr, NatType3)
		if err != nil {
			log.WithFields(log.Fields{
				"laddr": laddr,
				"raddr": raddr,
				"error": err,
			}).Debug("send restricted port nat request failed")
		} else {
			log.WithFields(log.Fields{
				"laddr": laddr,
				"raddr": raddr,
			}).Info("send restricted port nat request success")
		}
	}()
}

func (s *Server) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	ln, err := ListenTcp(ctx, s.Host1+":"+strconv.Itoa(s.Port1))
	if err != nil {
		log.Fatal(err)
	}
	s.cancel = cancel
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
	if s.cancel != nil {
		s.cancel()
	}
}
