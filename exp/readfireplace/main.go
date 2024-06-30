package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// func main() {
// 	// Define the server address
// 	serverAddr := net.UDPAddr{
// 		IP:   net.ParseIP("10.0.0.40"),
// 		Port: 3300,
// 	}

// 	// Create a UDP connection
// 	conn, err := net.DialUDP("udp", nil, &serverAddr)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer conn.Close()

// 	// Message to send
// 	message := []byte("Hello, UDP server!")

// 	// Send the message
// 	_, err = conn.Write(message)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Buffer to hold the response
// 	buffer := make([]byte, 1024)

// 	// Read the response
// 	n, _, err := conn.ReadFromUDP(buffer)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Print the response
// 	fmt.Printf("Received: %s\n", string(buffer[:n]))
// }

type Fire struct {
	UDP_PORT int
	ip       string
	server   *net.UDPConn
	mu       sync.Mutex
}

type FireStatus struct {
	status      bool
	fanBoost    bool
	flameEffect bool
	desiredTemp int
	roomTemp    int
}

func NewFireplace(ip string) *Fire {
	return &Fire{
		UDP_PORT: 3300,
		ip:       ip,
	}
}

func (f *Fire) sendRequest(message string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.server == nil {
		addr := net.UDPAddr{
			Port: f.UDP_PORT,
			IP:   net.ParseIP(f.ip),
		}
		conn, err := net.DialUDP("udp4", nil, &addr)
		if err != nil {
			return nil, err
		}
		f.server = conn

		// Close the server after 1 second
		go func() {
			time.Sleep(1 * time.Second)
			f.mu.Lock()
			f.server.Close()
			f.server = nil
			f.mu.Unlock()
		}()
	}

	m, err := hex.DecodeString(message)
	if err != nil {
		return nil, err
	}

	_, err = f.server.Write(m)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 1024)
	n, _, err := f.server.ReadFromUDP(buffer)
	if err != nil {
		return nil, err
	}

	return buffer[:n], nil
}

func (f *Fire) GetStatus() (*FireStatus, error) {
	const STATUS_PLEASE = "473100000000000000000000003146"
	m, err := f.sendRequest(STATUS_PLEASE)
	if err != nil {
		return nil, err
	}

	status := &FireStatus{
		status:      readBufferAsBool(m, 4, 5),
		fanBoost:    readBufferAsBool(m, 5, 6),
		flameEffect: readBufferAsBool(m, 6, 7),
		desiredTemp: readBufferAsInt(m, 7, 8),
		roomTemp:    readBufferAsInt(m, 8, 9),
	}
	return status, nil
}

func (f *Fire) SetOn() (bool, error) {
	const POWER_ON = "473900000000000000000000003146"
	_, err := f.sendRequest(POWER_ON)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (f *Fire) SetOff() (bool, error) {
	const POWER_OFF = "473A00000000000000000000003146"
	_, err := f.sendRequest(POWER_OFF)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (f *Fire) SetTemp(temp int) (bool, error) {
	if temp < 10 || temp > 31 {
		return false, nil
	}

	tempHex := decimalToHexString(temp)
	SET_TEMP := fmt.Sprintf("475701%s0000000000000000003146", tempHex)
	_, err := f.sendRequest(SET_TEMP)
	if err != nil {
		return false, err
	}
	return true, nil
}

func readBufferAsBool(buffer []byte, start, end int) bool {
	return buffer[start] != 0
}

func readBufferAsInt(buffer []byte, start, end int) int {
	return int(buffer[start])
}

func decimalToHexString(value int) string {
	return fmt.Sprintf("%02X", value)
}

func main() {
	fireplace := NewFireplace("10.0.0.40")
	slog.Info("Getting fireplace status")

	status, err := fireplace.GetStatus()
	if err != nil {
		slog.Error("Error getting status", "error", err)
		return
	}

	slog.Info("Status", "status", status)

	// success, err := fire.SetOn()
	// if err != nil {
	// 	fmt.Println("Error setting on:", err)
	// 	return
	// }
	// fmt.Println("Set On:", success)

	// success, err = fire.SetOff()
	// if err != nil {
	// 	fmt.Println("Error setting off:", err)
	// 	return
	// }
	// fmt.Println("Set Off:", success)

	// success, err = fire.SetTemp(25)
	// if err != nil {
	// 	fmt.Println("Error setting temp:", err)
	// 	return
	// }

	// fmt.Println("Set Temp:", success)
}
