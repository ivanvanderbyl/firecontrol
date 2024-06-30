package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
)

func main() {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("10.0.0.40"),
		Port: 3300,
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer conn.Close()

	packetHex := "473100000000000000000000003146"
	message, err := hex.DecodeString(packetHex)
	if err != nil {
		slog.Error("Error decoding hex string", "error", err)
		return
	}
	_, err = conn.Write(message)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Received response: %s\n", string(buffer[:n]))

	// // Define the server address
	// addr, err := net.ResolveUDPAddr("udp", "10.0.0.40:3300")
	// if err != nil {
	// 	fmt.Println("Error resolving UDP address:", err)
	// 	return
	// }

	// slog.Info("Connecting to fireplace", "address", addr.String())

	// // Create a UDP connection
	// conn, err := net.DialUDP("udp4", nil, addr)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer conn.Close()

	// conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Message to send

	// m, err := hex.DecodeString("473100000000000000000000003146")
	// packetHex := "473100000000000000000000003146"
	// m, err := hex.DecodeString(packetHex)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// slog.Info("Sending message", "message", m)

	// // Send the message
	// _, err = conn.Write(m)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// for {
	// 	// Buffer to hold the response
	// 	buffer := make([]byte, 1024)

	// 	slog.Info("Waiting for response...")

	// 	// Read the response
	// 	_, _, err := conn.ReadFrom(buffer)
	// 	if err != nil {
	// 		slog.Error("Error reading response", "error", err)
	// 		continue
	// 	}

	// 	slog.Info("Received", "response", buffer)
	// }

	// // Buffer to hold the response
	// buffer := make([]byte, 1024)

	// slog.Info("Waiting for response...")

	// // data, err := bufio.NewReader(conn).ReadString('\n')
	// // if err != nil {
	// // 	fmt.Println(err)
	// // 	return
	// // }

	// // Read the response
	// n, _, err := conn.ReadFrom(buffer)
	// if err != nil {
	// 	slog.Error("Error reading response", "error", err)
	// 	return
	// }

	// slog.Info("Received", "bytes", n)

	// data := buffer[:n]

	// // Print the response
	// slog.Info("Received", "response", data)

}
