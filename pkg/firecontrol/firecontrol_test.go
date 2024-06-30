package firecontrol

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePacket(t *testing.T) {
	a := assert.New(t)

	tests := []struct {
		command        CommandCode
		data           []byte
		expectedPacket []byte
	}{
		{
			command:        CommandSearchForFireplaces,
			expectedPacket: mustDecode("475000000000000000000000005046"),
		},
		{
			command:        CommandPowerOn,
			expectedPacket: mustDecode("473900000000000000000000003946"),
		},
		{
			command:        CommandSetTemperature,
			data:           []byte{22},
			expectedPacket: mustDecode("475701160000000000000000006e46"),
		},
	}

	for _, test := range tests {
		packet := marshalCommandPacket(test.command, test.data)
		a.EqualValues(test.expectedPacket, packet)
	}
}

func TestParseFireplaceResponse(t *testing.T) {
	a := assert.New(t)

	fp, err := parseFireplaceResponse(mustDecode("4790040001A4ED06FE000000002A46"))
	a.NoError(err)
	a.EqualValues(Fireplace{
		Serial: 107757,
		PIN:    1790,
	}, fp)
}

func TestIsValidResponse(t *testing.T) {
	a := assert.New(t)

	a.True(isValidCRC(mustDecode("478006000100001B1800000000BA46")))
	a.True(isValidResponse(mustDecode("4790040001A4ED06FE000000002A46")))
	a.False(isValidResponse(mustDecode("4790040001A4ED06FE000000002A47")))
	a.False(isValidResponse(mustDecode("4790040001A4ED06FE000000002A45")))
	a.False(isValidResponse(mustDecode("4790040001A4ED06FE000000002A")))
}

func TestParseStatusResponse(t *testing.T) {
	a := assert.New(t)

	status, err := parseStatusResponse(mustDecode("478006000100001B1800000000BA46"))
	a.NoError(err)
	a.EqualValues(Status{
		IsOn:               true,
		HasTimers:          false,
		FlameEffectIsOn:    false,
		FanBoostIsOn:       false,
		DesiredTempertaure: 27,
		RoomTemperature:    24,
	}, status)
}

func mustDecode(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
