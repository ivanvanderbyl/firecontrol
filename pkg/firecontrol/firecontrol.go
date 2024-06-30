package firecontrol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"time"
)

// Search for fireplaces on the local network
// 475000000000000000000000005046

// I AM A FIRE response ID
// 4790040001a4ed06fe000000002a46

// STATUS response
// 0x478006000100001B1800000000BA46

// Fireplace represents a fireplace on the local network
type Fireplace struct {
	Serial uint32
	PIN    uint16
	Status *Status
}

type Status struct {
	HasTimers          bool
	IsOn               bool
	FanBoostIsOn       bool
	FlameEffectIsOn    bool
	DesiredTempertaure uint8
	RoomTemperature    uint8
}

var ErrInvalidTemperature = errors.New("invalid temperature")
var ErrInvalidResponse = errors.New("invalid response packet")

type CommandCode uint8
type ResponseCode uint8

const (
	// Remote commands for the fireplace
	CommandStatusPlease        CommandCode = 0x31
	CommandPowerOn             CommandCode = 0x39
	CommandPowerOff            CommandCode = 0x3A
	CommandSearchForFireplaces CommandCode = 0x50
	CommandFanBoostOn          CommandCode = 0x37
	CommandFanBoostOff         CommandCode = 0x38
	CommandFlameEffectOn       CommandCode = 0x56
	CommandFlameEffectOff      CommandCode = 0x55
	CommandSetTemperature      CommandCode = 0x57

	// Responses from the fireplace
	ResponseStatus            ResponseCode = 0x80 // Response to status request
	ResponsePowerOnAck        ResponseCode = 0x8D // Acknowledgement of power on
	ResponsePowerOffAck       ResponseCode = 0x8F // Acknowledgement of power off
	ResponseFanBoostOnAck     ResponseCode = 0x89 // Acknowledgement of fan boost on
	ResponseFanBoostOffAck    ResponseCode = 0x8B // Acknowledgement of fan boost off
	ResponseFlameEffectOnAck  ResponseCode = 0x61 // Acknowledgement of flame effect on
	ResponseFlameEffectOffAck ResponseCode = 0x60 // Acknowledgement of flame effect off
	ResponseTemperatureAck    ResponseCode = 0x66 // Acknowledgement of temperature change
	ResponseIAmAFire          ResponseCode = 0x90 // Response to search command

	packetSize = 15
	startByte  = 0x47
	endByte    = 0x46

	// Temperature range for the fireplace based on v0.3 of spec
	minTemperature = 3
	maxTemperature = 31

	// Fireplace broadcast port
	fireplacePort = 3300
)

func (f *Fireplace) PowerOn() error {
	return nil
}

func (f *Fireplace) SetTemperature(newTemp int) error {
	if newTemp < minTemperature || newTemp > maxTemperature {
		return ErrInvalidTemperature
	}

	return nil
}

func SearchForFireplaces() ([]*Fireplace, error) {
	conn, err := net.DialUDP("udp4",
		nil,
		&net.UDPAddr{Port: fireplacePort, IP: net.ParseIP("255.255.255.255")},
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// listenAddr, err := net.ResolveUDPAddr("udp4", ":3300")
	listenAddr := net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), fireplacePort))

	// Create a UDP connection to listen for incoming packets
	listener, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		fmt.Println("Error listening on UDP:", err)
		return nil, err
	}
	defer listener.Close()

	listener.SetDeadline(time.Now().Add(3 * time.Second))
	listener.SetReadBuffer(1024)

	// Send search command
	searchPacket := marshalCommandPacket(CommandSearchForFireplaces, []byte{})
	_, err = conn.Write(searchPacket)
	if err != nil {
		return nil, err
	}

	// Wait for responses
	fireplaces := make([]*Fireplace, 0)
	for {
		buffer := make([]byte, 1024)
		n, _, err := listener.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			log.Printf("Error reading from UDP: %v", err)
			return nil, err
		}

		cmd, err := UnmarshalCommandPacket(buffer[:n])
		if err != nil {
			return nil, err
		}

		if cmd.CommandID != ResponseIAmAFire {
			continue
		}

		fp, err := parseFireplaceResponse(buffer[:n])
		if err != nil {
			return nil, err
		}

		fireplaces = append(fireplaces, &fp)
	}

	return fireplaces, nil
}

type Command struct {
	StartByte byte
	CommandID ResponseCode
	DataSize  uint8
	Data      [10]byte
	CRC       uint8
	EndByte   byte
}

func calculateCRC(data []byte) byte {
	var sum byte
	for _, b := range data {
		sum += b
	}
	return sum
}

// func createSearchCommand(command CommandCode, data [10]byte) []byte {
// 	cmd := Command{
// 		StartByte: startByte,
// 		CommandID: command,
// 		DataSize:  byte(len(data)),
// 		Data:      data,
// 		EndByte:   endByte,
// 	}
// 	cmd.CRC = calculateCRC([]byte{cmd.CommandID, cmd.DataSize})
// 	buf := new(bytes.Buffer)
// 	binary.Write(buf, binary.BigEndian, cmd)
// 	return buf.Bytes()
// }

func marshalCommandPacket(command CommandCode, data []byte) []byte {
	packet := make([]byte, packetSize)
	packet[0] = startByte
	packet[1] = byte(command)
	packet[2] = byte(len(data)) // DataSize

	// Copy data into packet
	copy(packet[3:], data)

	// Close packet
	packet[13] = calculateCRC(packet[1:14])
	packet[14] = endByte

	return packet
}

func UnmarshalCommandPacket(packet []byte) (*Command, error) {
	if !isValidResponse(packet) {
		return nil, ErrInvalidResponse
	}

	cmd := new(Command)
	err := binary.Read(bytes.NewReader(packet), binary.BigEndian, cmd)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func isValidResponse(packet []byte) bool {
	return len(packet) == 15 && // packetSize must be 15 bytes
		packet[0] == startByte && // Start byte must be 0x47 G
		packet[14] == endByte && // End byte must be 0x46 F
		isValidCRC(packet) // CRC must be valid
}

func isValidCRC(packet []byte) bool {
	crc := calculateCRC(packet[1:12])
	return crc == packet[13]
}

type foundFireplacePayload struct {
	DataLength uint8
	Serial     uint32
	PIN        uint16
}

func parseFireplaceResponse(packet []byte) (Fireplace, error) {
	if !isValidResponse(packet) {
		return Fireplace{}, ErrInvalidResponse
	}

	data := packet[2 : 2+10] // Data is 10 bytes long
	payload := foundFireplacePayload{}
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &payload)
	if err != nil {
		return Fireplace{}, err
	}

	return Fireplace{
		Serial: payload.Serial,
		PIN:    payload.PIN,
	}, nil
}

func parseStatusResponse(packet []byte) (*Status, error) {
	if !isValidResponse(packet) {
		return nil, ErrInvalidResponse
	}

	data := packet[3 : 3+10] // Data is 10 bytes long
	payload := new(Status)
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}
