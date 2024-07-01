package firecontrol

import (
	"errors"
	"net"
	"time"
)

const (
	readBufferSize = 128
	timeout        = 3 * time.Second
)

func (f *Fireplace) rpc(command CommandCode, data []byte) (FireplaceData, error) {
	if f.Addr == nil {
		return nil, errors.New("fireplace address is nil")
	}

	conn, err := net.DialUDP("udp4", &net.UDPAddr{Port: fireplacePort}, f.Addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	statusPacket := marshalCommandPacket(command, data)
	n, err := conn.Write(statusPacket)
	if err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(timeout))
	conn.SetReadBuffer(readBufferSize)

	buffer := make([]byte, readBufferSize)
	_, _, err = conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, err
	}

	resp, err := UnmarshalCommandPacket(buffer[:n])
	if err != nil {
		return nil, err
	}

	payload, err := handleResponse(resp)
	if err != nil {
		return nil, err
	}

	return payload, nil
}
