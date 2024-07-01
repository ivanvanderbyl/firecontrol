package firecontrol_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ivanvanderbyl/escea-fireplace/pkg/firecontrol"
	"github.com/stretchr/testify/assert"
)

type Command[T any] struct {
	StartByte byte
	CommandID firecontrol.CommandCode
	DataSize  uint8
	Data      T
	CRC       uint8
	EndByte   byte
}

type Status struct {
	IsOn        bool
	Temperature uint8
	_           [8]byte
}

func Marshal[T any](cmd Command[T]) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, cmd)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Unmarshal[T any](data []byte) (Command[T], error) {
	cmd := Command[T]{}
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &cmd)
	if err != nil {
		return cmd, err
	}
	return cmd, nil
}

func TestMarshalNested(t *testing.T) {
	a := assert.New(t)
	cmd := Command[Status]{
		StartByte: 0x47,
		CommandID: firecontrol.CommandStatusPlease,
		DataSize:  0x06,
		Data: Status{
			IsOn:        true,
			Temperature: 27,
		},
		CRC:     0xBA,
		EndByte: 0x46,
	}

	result, err := Marshal(cmd)
	a.NoError(err)
	a.Equal([]byte{0x47, 0x31, 0x6, 0x1, 0x1b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xba, 0x46}, result)

	unmarshalled, err := Unmarshal[Status](result)
	a.NoError(err)
	a.Equal(cmd, unmarshalled)

	a.Equal(uint8(27), unmarshalled.Data.Temperature)
}

type Raw [10]byte

func TestMarshalBytes(t *testing.T) {
	a := assert.New(t)

	cmd := Command[Raw]{
		StartByte: 0x47,
		CommandID: firecontrol.CommandStatusPlease,
		DataSize:  0x06,
		Data:      Raw{0x1, 0x1b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		CRC:       0xBA,
		EndByte:   0x46,
	}

	result, err := Marshal(cmd)
	a.NoError(err)
	a.Equal([]byte{0x47, 0x31, 0x6, 0x1, 0x1b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xba, 0x46}, result)

	unmarshalled, err := Unmarshal[Raw](result)
	a.NoError(err)
	a.Equal(cmd, unmarshalled)
}

func TestMarshalBytesToStatus(t *testing.T) {
	a := assert.New(t)

	original := Command[Raw]{
		StartByte: 0x47,
		CommandID: firecontrol.CommandStatusPlease,
		DataSize:  0x06,
		Data:      Raw{0x1, 0x1b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		CRC:       0xBA,
		EndByte:   0x46,
	}

	result, err := Marshal(original)
	a.NoError(err)
	a.Equal([]byte{0x47, 0x31, 0x6, 0x1, 0x1b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xba, 0x46}, result)

	unmarshalled, err := Unmarshal[Status](result)
	a.NoError(err)
	a.Equal(uint8(27), unmarshalled.Data.Temperature)
}
