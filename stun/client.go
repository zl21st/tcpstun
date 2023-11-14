package stun

import (
	"context"
	"encoding/gob"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Client struct {
	ServerHost string // stun server host
	ServerPort int    // stun server port
	LocalAddr  string // local address to listen on
	Timeout    int    // connect timeout in seconds
	Basic      bool
	Debug      bool
	Verbose    bool
	natType    string
	mappedHost string
	mappedPort int
	lock       sync.Mutex
	log        *logrus.Logger
	cancel     context.CancelFunc
}

type NatResult struct {
	NatType    string
	MappedHost string
	MappedPort int
	LocalHost  string
	LocalPort  int
}

func NewClient(saddr, laddr string, timeout int) (*Client, error) {
	host, portStr, err := net.SplitHostPort(saddr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		ServerHost: host,
		ServerPort: port,
		LocalAddr:  laddr,
		Timeout:    timeout,
	}
	if err := c.Init(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) Init() error {
	if c.ServerHost == "" {
		return fmt.Errorf("ServerHost is empty")
	}

	if c.ServerPort == 0 {
		c.ServerPort = 3478
	}

	if c.Timeout == 0 {
		c.Timeout = 3
	}

	ip, err := GetOutboundIP()
	if err != nil {
		return err
	}

	if c.LocalAddr == "" || strings.HasPrefix(c.LocalAddr, ":") {
		c.LocalAddr = ip + c.LocalAddr
	}

	if !strings.Contains(c.LocalAddr, ":") {
		c.LocalAddr += ":0"
	}

	c.natType = NatTypeBlocked

	if c.Debug {
		c.log = logrus.New()
		c.log.SetFormatter(&logrus.TextFormatter{
			DisableColors:   false,
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
		c.log.SetLevel(logrus.DebugLevel)
	}

	return nil
}

func (c *Client) processConnection(conn net.Conn) {
	if c.log != nil {
		c.log.Debugf("connection accepted from %s", conn.RemoteAddr())
	}
	defer conn.Close()

	var req ServerRequest
	err := gob.NewDecoder(conn).Decode(&req)
	if err != nil {
		if c.log != nil {
			c.log.WithFields(logrus.Fields{
				"error": err,
			}).Debug("decode server request failed")
		}
		return
	}
	if c.log != nil {
		c.log.WithFields(logrus.Fields{
			"request": req,
		}).Debug("decode server request success")
	}

	var res ClienResponse
	err = gob.NewEncoder(conn).Encode(res)
	if err != nil && c.log != nil {
		c.log.WithFields(logrus.Fields{
			"error": err,
		}).Debug("encode client response failed")
	}

	// update nat type
	c.lock.Lock()
	defer c.lock.Unlock()
	c.natType = CompareNatType(c.natType, req.Type)
}

func (c *Client) detectNatType() error {
	// create a rpc server on localPort
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	ln, err := ListenTcp(ctx, c.LocalAddr)
	if err != nil {
		if c.log != nil {
			c.log.WithFields(logrus.Fields{
				"error": err,
			}).Debug("listen failed")
		}
		return err
	}
	if c.log != nil {
		c.log.WithFields(logrus.Fields{
			"localAddr": ln.Addr(),
		}).Debug("listen success")
	}
	c.LocalAddr = ln.Addr().String()

	go func() {
		for {
			// accept a connection
			conn, err := ln.Accept()
			if err != nil {
				if c.log != nil {
					c.log.WithFields(logrus.Fields{
						"error": err,
					}).Debug("accept failed")
				}
				break
			}

			go c.processConnection(conn)
		}
		defer ln.Close()
	}()

	return nil
}

func (c *Client) PrintResult() {
	hint := ""
	if c.Basic {
		hint = "(NAT Type detection not enabled in basic mode)"
	}
	fmt.Println("NAT Type:", c.natType, hint)
	if c.Verbose {
		host := strings.Split(c.LocalAddr, ":")[0]
		port, _ := strconv.Atoi(strings.Split(c.LocalAddr, ":")[1])
		fmt.Println("Local IP:", host)
		fmt.Println("Local Port:", port)
	}
	if c.mappedHost != "" {
		fmt.Println("External IP:", c.mappedHost)
		fmt.Println("External Port:", c.mappedPort)
	}
}

func (c *Client) Discover() (*NatResult, error) {
	if err := c.run(); err != nil {
		return nil, err
	}

	if !strings.Contains(c.LocalAddr, ":") {
		// should not happen
		return nil, fmt.Errorf("invalid local address")
	}

	host, portStr, err := net.SplitHostPort(c.LocalAddr)
	if err != nil {
		// should not happen
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		// should not happen
		return nil, err
	}

	return &NatResult{
		NatType:    c.natType,
		MappedHost: c.mappedHost,
		MappedPort: c.mappedPort,
		LocalHost:  host,
		LocalPort:  port,
	}, nil
}

func (c *Client) run() error {
	if !c.Basic {
		err := c.detectNatType()
		if err != nil {
			return err
		}

		defer func() {
			if c.cancel != nil {
				c.cancel()
			}
		}()
	}

	conn, err := DialTcp(c.LocalAddr, c.ServerHost+":"+strconv.Itoa(c.ServerPort), c.Timeout)
	if err != nil {
		if c.log != nil {
			c.log.WithFields(logrus.Fields{
				"error": err,
			}).Debug("dial server failed")
		}
		return err
	}

	// send the request
	var req ClientRequest
	req.LocalHost = conn.LocalAddr().(*net.TCPAddr).IP.String()
	req.LocalPort = conn.LocalAddr().(*net.TCPAddr).Port
	req.Type = RequestType2
	if c.Basic {
		req.Type = RequestType1
	}

	err = gob.NewEncoder(conn).Encode(req)
	if err != nil {
		if c.log != nil {
			c.log.WithFields(logrus.Fields{
				"request": req,
				"error":   err,
			}).Debug("encode client request failed")
		}
		conn.Close()
		return err
	}
	if c.log != nil {
		c.log.WithFields(logrus.Fields{
			"request": req,
		}).Debug("encode client request success")
	}

	// read the response
	var res ServerResponse
	err = gob.NewDecoder(conn).Decode(&res)
	if err != nil {
		if c.log != nil {
			c.log.WithFields(logrus.Fields{
				"error": err,
			}).Debug("decode server response failed")
		}
		conn.Close()
		return err
	}
	if c.log != nil {
		c.log.WithFields(logrus.Fields{
			"response": res,
		}).Debug("decode server response success")
	}
	conn.Close()

	c.mappedHost = res.ClientMappedHost
	c.mappedPort = res.ClientMappedPort
	c.natType = NatType4
	if res.ClientMappedHost == res.ClientLocalHost && res.ClientMappedPort == res.ClientLocalPort {
		c.natType = NatType0
		return nil
	}

	if c.Basic {
		return nil
	}

	// wait for timeout
	for i := 0; i < c.Timeout+1; i++ {
		time.Sleep(time.Second)
		c.lock.Lock()
		if c.natType == NatType1 {
			return nil
		}
		c.lock.Unlock()
	}

	return nil
}

func (c *Client) Run() {
	c.run()
}
