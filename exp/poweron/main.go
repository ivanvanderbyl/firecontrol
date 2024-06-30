package main

import (
	"encoding/hex"
	"fmt"
	"net"
)

func main() {
	// Define the UDP address and port
	udpAddr, err := net.ResolveUDPAddr("udp", "10.0.0.40:3300")
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
		return
	}

	// Create a UDP connection
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		fmt.Println("Error dialing UDP:", err)
		return
	}
	defer conn.Close()

	// Define the packet to turn on the fireplace (example packet)
	packetHex := "473900000000000000000000003146"
	packet, err := hex.DecodeString(packetHex)
	if err != nil {
		fmt.Println("Error decoding hex string:", err)
		return
	}

	// Send the packet
	_, err = conn.Write(packet)
	if err != nil {
		fmt.Println("Error sending UDP packet:", err)
		return
	}

	fmt.Println("Turn on command sent successfully")
}
