package fastly

import (
	"bytes"
	"encoding/json"
)

type Compatibool bool

var _ json.Unmarshaler = new(Compatibool)

// Occasionally these bools come down from fastly in '0'/'1', or even 0/1 form.
func (b *Compatibool) UnmarshalJSON(t []byte) error {
	if bytes.Equal(t, []byte("1")) || string(t) == "\"1\"" {
		*b = Compatibool(true)
	}
	return nil
}

type BatchOperation int

const (
	_                                   = iota
	BatchOperationUpdate BatchOperation = iota
	BatchOperationCreate
	BatchOperationDelete
)

func (s *BatchOperation) UnmarshalText(b []byte) error {
	switch string(b) {
	case "update":
		*s = BatchOperationUpdate
	case "create":
		*s = BatchOperationCreate
	case "delete":
		*s = BatchOperationDelete
	}
	return nil
}

func (s *BatchOperation) MarshalText() ([]byte, error) {
	switch *s {
	case BatchOperationUpdate:
		return []byte("update"), nil
	case BatchOperationCreate:
		return []byte("create"), nil
	case BatchOperationDelete:
		return []byte("delete"), nil
	}
	return nil, nil
}
