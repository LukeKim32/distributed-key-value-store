package tools

import (
	"net"
)

func GetCurrentServerIP() (string, error){
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "",err
	}

	var currentIP string

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				currentIP = ipnet.IP.String()
			}
		}
	}

	return currentIP, nil
}