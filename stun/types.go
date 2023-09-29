package stun

type ClientRequest struct {
	Type      string
	LocalHost string
	LocalPort int
}

type ServerResponse struct {
	ClientLocalHost  string
	ClientLocalPort  int
	ClientMappedHost string
	ClientMappedPort int
	ServerHost1      string
	ServerHost2      string
	ServerPort1      int
	ServerPort2      int
}

type ServerRequest struct {
	Type string
}

type ClienResponse struct {
}
