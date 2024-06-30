package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

const (
	udpPort           = 3300
	searchCommandID   = 0x50
	responseCommandID = 0x80
	startByte         = 0x47
	endByte           = 0x46
	broadcastAddress  = "10.0.0.255"
)

type Command struct {
	StartByte byte
	CommandID byte
	DataSize  byte
	Data      [10]byte
	CRC       byte
	EndByte   byte
}

func calculateCRC(data []byte) byte {
	var sum byte
	for _, b := range data {
		sum += b
	}
	return sum
}

func createSearchCommand() []byte {
	cmd := Command{
		StartByte: startByte,
		CommandID: searchCommandID,
		DataSize:  0,
		EndByte:   endByte,
	}
	cmd.CRC = calculateCRC([]byte{cmd.CommandID, cmd.DataSize})
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, cmd)
	return buf.Bytes()
}

func listenForResponses(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("No more responses.")
				return
			}
			fmt.Println("Error reading from UDP:", err)
			continue
		}
		if n >= 15 && buffer[0] == startByte && buffer[1] == responseCommandID && buffer[14] == endByte {
			fmt.Printf("Received response from %s: %x\n", addr, buffer[:n])
		}

		fmt.Printf("\nFound fireplace at %s", addr.String())
	}
}

func main() {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", broadcastAddress, udpPort))
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
		return
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		fmt.Println("Error listening on UDP:", err)
		return
	}
	defer conn.Close()

	searchCommand := createSearchCommand()
	fmt.Println(hex.EncodeToString(searchCommand))

	_, err = conn.WriteToUDP(searchCommand, addr)
	if err != nil {
		fmt.Println("Error sending search command:", err)
		return
	}

	fmt.Println("Sent SEARCH_FOR_FIRES command, waiting for responses...")
	listenForResponses(conn)
}
