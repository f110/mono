package inkbird

import "tinygo.org/x/bluetooth"

func parseAddress(id string) (bluetooth.Address, error) {
	m, err := bluetooth.ParseMAC(id)
	if err != nil {
		return bluetooth.Address{}, xerrors.Errorf(": %w", err)
	}
	return bluetooth.Address{MACAddress: m}, nil
}
