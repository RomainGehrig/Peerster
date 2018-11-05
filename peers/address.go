package peers

import (
	"fmt"
	"net"
)

type PeerAddress interface {
	ToUDPAddr() *net.UDPAddr
	String() string
}

type StringAddress struct {
	Name string
}

type UDPAddress struct {
	UDPAddr *net.UDPAddr
}

// Can resolve names to ip addresses
func ResolvePeerAddress(name string) PeerAddress {
	return UDPAddress{StringAddress{name}.ToUDPAddr()}
}

func AsStringAddresses(strs ...string) []PeerAddress {
	addrs := make([]PeerAddress, 0)
	for _, str := range strs {
		addrs = append(addrs, StringAddress{str})
	}
	return addrs
}

func (addr UDPAddress) ToUDPAddr() *net.UDPAddr {
	return addr.UDPAddr
}

func (name StringAddress) ToUDPAddr() *net.UDPAddr {
	udpAddr, err := net.ResolveUDPAddr("udp4", name.Name)
	if err != nil {
		fmt.Println("Error when resolving", name, ":", err)
	}
	return udpAddr
}

func (addr UDPAddress) String() string {
	return addr.UDPAddr.String()
}

func (name StringAddress) String() string {
	return name.Name
}
