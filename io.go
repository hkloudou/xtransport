package xtransport

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"unsafe"
)

// Write marshals v and writes it to w. Supported types are nil, string,
// []byte, io.WriterTo, encoding.BinaryMarshaler and json.Marshaler,
// checked in that order.
func Write(w io.Writer, v interface{}) (n int, err error) {
	switch v := v.(type) {
	case nil:
		return 0, nil
	case string:
		return WriteString(w, v)
	case []byte:
		return WriteBytes(w, v)
	case io.WriterTo:
		nb, err := v.WriteTo(w)
		return int(nb), err
	case encoding.BinaryMarshaler:
		b, err := v.MarshalBinary()
		if err != nil {
			return 0, err
		}
		return WriteBytes(w, b)
	case json.Marshaler:
		b, err := v.MarshalJSON()
		if err != nil {
			return 0, err
		}
		return WriteBytes(w, b)
	default:
		return 0, fmt.Errorf(
			"xtransport: can't marshal %T (implement io.WriterTo or encoding.BinaryMarshaler)", v)
	}
}

func WriteBytes(w io.Writer, data []byte) (n int, err error) {
	return w.Write(data)
}

func WriteString(w io.Writer, data string) (n int, err error) {
	return w.Write(stringToBytes(data))
}

// stringToBytes converts a string to a byte slice without copying.
// The returned slice must not be modified.
func stringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
