package xtransport

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"unsafe"
)

func Write(w io.Writer, v interface{}) (n int, err error) {
	switch v := v.(type) {
	case nil:
		return WriteString(w, "")
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
			"redis: can't marshal %T (implement encoding.BinaryMarshaler)", v)
	}
}

func WriteBytes(w io.Writer, data []byte) (n int, err error) {
	return w.Write(data)
}

func WriteString(w io.Writer, data string) (n int, err error) {
	return w.Write(stringToBytes(data))
}

//https://github.com/go-redis/redis/blob/master/internal/util/unsafe.go
// stringToBytes converts string to byte slice.
func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}
