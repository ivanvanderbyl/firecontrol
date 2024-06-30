package main

import (
	"fmt"
	"net"
	"time"
)

const (
	udpPort         = 3300
	startByte       = 0x47 // 'G'
	endByte         = 0x46 // 'F'
	searchCommandID = 0x50
	packetSize      = 15
)

type Fireplace struct {
	IP     net.IP
	Serial string
	PIN    string
}

func main() {
	fireplaces, err := searchForFireplaces()
	if err != nil {
		fmt.Printf("Error searching for fireplaces: %v\n", err)
		return
	}

	for _, fp := range fireplaces {
		fmt.Printf("Found fireplace - IP: %s, Serial: %s, PIN: %s\n", fp.IP, fp.Serial, fp.PIN)
	}
}

func searchForFireplaces() ([]Fireplace, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	broadcastAddr := &net.UDPAddr{
		IP: net.IPv4bcast,
		// IP:   net.ParseIP("10.0.0.255"),
		Port: udpPort,
	}

	fmt.Printf("Searching for fireplaces on %s:%d\n", broadcastAddr.IP, broadcastAddr.Port)

	packet := buildSearchPacket()
	_, err = conn.WriteToUDP(packet, broadcastAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to send search packet: %w", err)
	}

	var fireplaces []Fireplace
	responseBuffer := make([]byte, 1024)
	deadline := time.Now().Add(30 * time.Second)
	conn.SetReadDeadline(deadline)

	for time.Now().Before(deadline) {
		n, remoteAddr, err := conn.ReadFromUDP(responseBuffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return nil, fmt.Errorf("error reading response: %w", err)
		}

		if n == packetSize && isValidResponse(responseBuffer[:n]) {
			fp := parseFireplaceResponse(responseBuffer[:n], remoteAddr.IP)
			fireplaces = append(fireplaces, fp)
		}
	}

	return fireplaces, nil
}

func buildSearchPacket() []byte {
	packet := make([]byte, packetSize)
	packet[0] = startByte
	packet[1] = searchCommandID
	packet[2] = 0 // DataSize
	packet[14] = endByte

	// Calculate CRC
	var crc byte
	for i := 1; i <= 13; i++ {
		crc += packet[i]
	}
	packet[13] = crc

	return packet
}

func isValidResponse(packet []byte) bool {
	return packet[0] == startByte && packet[14] == endByte && packet[1] == 0x80 // I_AM_A_FIRE response ID
}

func parseFireplaceResponse(packet []byte, ip net.IP) Fireplace {
	dataSize := int(packet[2])
	data := packet[3 : 3+dataSize]

	// Assuming the Serial and PIN are stored as null-terminated strings
	serialEnd := 0
	for i, b := range data {
		if b == 0 {
			serialEnd = i
			break
		}
	}

	serial := string(data[:serialEnd])
	pin := string(data[serialEnd+1:])

	return Fireplace{
		IP:     ip,
		Serial: serial,
		PIN:    pin,
	}
}
