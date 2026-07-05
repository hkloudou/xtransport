package test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/hkloudou/xtransport"
)

type binaryType struct{ data []byte }

func (t *binaryType) MarshalBinary() ([]byte, error) {
	return t.data, nil
}

type writerToType struct{ data []byte }

func (t *writerToType) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(t.data)
	return int64(n), err
}

type jsonType struct{}

func (t *jsonType) MarshalJSON() ([]byte, error) {
	return []byte(`{"a":1}`), nil
}

func TestWrite(t *testing.T) {
	cases := []struct {
		name string
		v    interface{}
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"bytes", []byte{1, 2, 3}, "\x01\x02\x03"},
		{"writer to", &writerToType{data: []byte("wt")}, "wt"},
		{"binary marshaler", &binaryType{data: []byte("bin")}, "bin"},
		{"json marshaler", &jsonType{}, `{"a":1}`},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if _, err := xtransport.Write(&buf, c.v); err != nil {
			t.Errorf("%s: Write returned %v", c.name, err)
			continue
		}
		if buf.String() != c.want {
			t.Errorf("%s: wrote %q, want %q", c.name, buf.String(), c.want)
		}
	}
}

func TestWriteUnsupportedType(t *testing.T) {
	var buf bytes.Buffer
	_, err := xtransport.Write(&buf, struct{ X int }{1})
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if strings.Contains(err.Error(), "redis") {
		t.Fatalf("error message references the wrong library: %v", err)
	}
}
