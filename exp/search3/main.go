package main

import (
	"fmt"
	"net"

	"github.com/ivanvanderbyl/escea-fireplace/pkg/firecontrol"
)

func main() {
	conn, err := net.DialUDP("udp4",
		// &net.UDPAddr{Port: 3300},
		nil,
		&net.UDPAddr{Port: 3300, IP: net.ParseIP("255.255.255.255")},
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	fs, err := firecontrol.SearchForFireplaces(conn)
	if err != nil {
		panic(err)
	}

	for _, f := range fs {
		fmt.Println("Found Fireplace", f.PIN, f.Serial)
	}
}
