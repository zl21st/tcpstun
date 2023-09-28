package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/zl21st/tcpstun/common"
)

type Client struct {
	ServerHost string
	ServerPort int
	Timeout    int
	natType    string
	mappedHost string
	mappedPort int
	lock       sync.Mutex
}

func (c *Client) Check() {
	if c.ServerHost == "" {
		log.Fatal("ServerHost is empty")
	}

	c.natType = common.NatTypeBlocked
}

func (c *Client) processConnection(conn net.Conn) {
	log.Debugf("connection accepted from %s", conn.RemoteAddr())
	defer conn.Close()

	var req common.ServerRequest
	err := gob.NewDecoder(conn).Decode(&req)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("decode server request failed")
		return
	}
	log.WithFields(log.Fields{
		"request": req,
	}).Debug("decode server request success")

	var res common.ClienResponse
	err = gob.NewEncoder(conn).Encode(res)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("encode client response failed")
	}

	// update nat type
	c.lock.Lock()
	defer c.lock.Unlock()
	c.natType = common.CompareNatType(c.natType, req.Type)
}

func (c *Client) PrintResult() {
	fmt.Println("NAT Type:", c.natType)
	if c.mappedHost != "" {
		fmt.Println("External IP:", c.mappedHost)
		fmt.Println("External Port:", c.mappedPort)
	}
}

func main() {
	cln := Client{}
	flag.StringVar(&cln.ServerHost, "H", "", "server host")
	flag.IntVar(&cln.ServerPort, "P", 3478, "server port")
	flag.IntVar(&cln.Timeout, "O", 3, "timeout")
	laddr := flag.String("l", "", "local address, ip or ip:port")
	debug := flag.Bool("d", false, "enable debug mode")
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	if !strings.Contains(*laddr, ":") {
		*laddr = *laddr + ":0"
	}

	cln.Check()
	defer cln.PrintResult()

	conn, err := common.DialTcp(*laddr, cln.ServerHost+":"+strconv.Itoa(cln.ServerPort), cln.Timeout)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("dial server failed")
		return
	}

	// send the request
	var req common.ClientRequest
	req.LocalHost = conn.LocalAddr().(*net.TCPAddr).IP.String()
	req.LocalPort = conn.LocalAddr().(*net.TCPAddr).Port
	req.Type = common.RequestType2
	err = gob.NewEncoder(conn).Encode(req)
	if err != nil {
		log.WithFields(log.Fields{
			"request": req,
			"error":   err,
		}).Debug("encode client request failed")
		conn.Close()
		return
	}
	log.WithFields(log.Fields{
		"request": req,
	}).Debug("encode client request success")

	// read the response
	var res common.ServerResponse
	err = gob.NewDecoder(conn).Decode(&res)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("decode server response failed")
		conn.Close()
		return
	}
	log.WithFields(log.Fields{
		"response": res,
	}).Debug("decode server response success")
	conn.Close()

	cln.mappedHost = res.ClientMappedHost
	cln.mappedPort = res.ClientMappedPort
	cln.natType = common.NatType4
	if res.ClientMappedHost == res.ClientLocalHost && res.ClientMappedPort == res.ClientLocalPort {
		cln.natType = common.NatType0
		return
	}

	// create a rpc server on localPort
	ln, err := common.ListenTcp(res.ClientLocalHost + ":" + strconv.Itoa(res.ClientLocalPort))
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("listen failed")
		return
	}

	go func() {
		for {
			// accept a connection
			conn, err := ln.Accept()
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Debug("accept failed")
				return
			}

			go cln.processConnection(conn)
		}
	}()

	// wait for timeout
	for i := 0; i < cln.Timeout+1; i++ {
		time.Sleep(time.Second)
		cln.lock.Lock()
		if cln.natType == common.NatType1 {
			return
		}
		cln.lock.Unlock()
	}
}
