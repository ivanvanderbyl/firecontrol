package firecontrol

import "fmt"

type (
	PowerOnAck  struct{}
	PowerOffAck struct{}
	SetTempAck  struct{}
)

func (f *PowerOnAck) isFireplaceData()  {}
func (f *PowerOffAck) isFireplaceData() {}
func (f *SetTempAck) isFireplaceData()  {}

func (f *Fireplace) PowerOn() error {
	data, err := f.rpc(CommandPowerOn, nil)
	if err != nil {
		return err
	}

	_, ok := data.(*PowerOnAck)
	if !ok {
		return fmt.Errorf("unexpected data type: %T", data)
	}

	return nil
}

func (f *Fireplace) PowerOff() error {
	data, err := f.rpc(CommandPowerOff, nil)
	if err != nil {
		return err
	}

	_, ok := data.(*PowerOffAck)
	if !ok {
		return fmt.Errorf("unexpected data type: %T", data)
	}

	return nil
}

// Refresh the status of the fireplace
func (f *Fireplace) Refresh() error {
	data, err := f.rpc(CommandStatusPlease, nil)
	if err != nil {
		return err
	}

	status, ok := data.(*Status)
	if !ok {
		return fmt.Errorf("unexpected data type: %T", data)
	}

	f.Status = status
	return nil
}

func (f *Fireplace) SetTemperature(newTemp int) error {
	if newTemp < minTemperature || newTemp > maxTemperature {
		return ErrInvalidTemperature
	}

	data, err := f.rpc(CommandSetTemperature, []byte{uint8(newTemp)})
	if err != nil {
		return err
	}

	_, ok := data.(*SetTempAck)
	if !ok {
		return fmt.Errorf("unexpected data type: %T", data)
	}

	return nil
}
