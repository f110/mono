package inkbird

import (
	"golang.org/x/xerrors"
	"tinygo.org/x/bluetooth"
)

func parseAddress(id string) (bluetooth.Address, error) {
	u, err := bluetooth.ParseUUID(id)
	if err != nil {
		return bluetooth.Address{}, xerrors.Errorf(": %w", err)
	}

	return bluetooth.Address{UUID: u}, err
}
