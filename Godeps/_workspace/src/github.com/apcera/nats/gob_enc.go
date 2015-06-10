// Copyright 2013 Apcera Inc. All rights reserved.

package nats

import (
	"bytes"
	"encoding/gob"
)

// A Go specific GOB Encoder implementation for EncodedConn
// This encoder will use the builtin encoding/gob to Marshal
// and Unmarshal most types, including structs.
type GobEncoder struct {
	// Empty
}

// FIXME(dlc) - This could probably be more efficient.

func (ge *GobEncoder) Encode(subject string, v interface{}) ([]byte, error) {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (ge *GobEncoder) Decode(subject string, data []byte, vPtr interface{}) (err error) {
	dec := gob.NewDecoder(bytes.NewBuffer(data))
	err = dec.Decode(vPtr)
	return
}
