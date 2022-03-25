package test

import (
	"testing"
)

type MyType struct{}

// var _ encoding.BinaryMarshaler = (*MyType)(nil)

func (t *MyType) MarshalBinary() ([]byte, error) {
	return []byte("hello"), nil
}

func TestXxx(t *testing.T) {
	// s := &MyType{}
	write(&MyType{})
}

func write(v interface{}) {
	// switch v := v.(type) {
	// case []byte:
	// 	log.Println("bytes")
	// case encoding.BinaryMarshaler:
	// 	// b, err := v.MarshalBinary()
	// 	// if err != nil {
	// 	// 	return err
	// 	// }
	// 	// return w.bytes(b)
	// 	log.Println("encoding.BinaryMarshaler")
	// case *MyType:
	// 	log.Println("Mytype")
	// }
}
