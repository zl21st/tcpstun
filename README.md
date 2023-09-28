# TCP NAT Type Test

This toolkit uses TCP protocol to detect NAT types. 

It is divided into a server and a client, which work together to detect the specific NAT type of the client's network. 

Currently, it can identify the following 5 types of NAT:
- NAT0: Public IP, no NAT
- NAT1: Full Cone NAT
- NAT2: Restricted Cone NAT
- NAT3: Port Restricted Cone NAT
- NAT4: Symmetric NAT

Note: This toolkit confirms to no RFC, the server and client must be used together.

## Server
## Client