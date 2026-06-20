package bencode

import (
	"testing"
)

func TestDecodeString(t *testing.T) {
	v, err := Decode([]byte("4:spam"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != String {
		t.Fatalf("expected string, got %s", v.Type)
	}
	if v.Str != "spam" {
		t.Fatalf("expected 'spam', got %q", v.Str)
	}
}

func TestDecodeEmptyString(t *testing.T) {
	v, err := Decode([]byte("0:"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Str != "" {
		t.Fatalf("expected empty string, got %q", v.Str)
	}
}

func TestDecodeInt(t *testing.T) {
	v, err := Decode([]byte("i42e"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Int {
		t.Fatalf("expected int, got %s", v.Type)
	}
	if v.Int != 42 {
		t.Fatalf("expected 42, got %d", v.Int)
	}
}

func TestDecodeNegativeInt(t *testing.T) {
	v, err := Decode([]byte("i-3e"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != -3 {
		t.Fatalf("expected -3, got %d", v.Int)
	}
}

func TestDecodeZeroInt(t *testing.T) {
	v, err := Decode([]byte("i0e"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 0 {
		t.Fatalf("expected 0, got %d", v.Int)
	}
}

func TestDecodeList(t *testing.T) {
	v, err := Decode([]byte("l4:spami42ee"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != List {
		t.Fatalf("expected list, got %s", v.Type)
	}
	if len(v.List) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(v.List))
	}
	if v.List[0].Str != "spam" {
		t.Fatalf("expected 'spam', got %q", v.List[0].Str)
	}
	if v.List[1].Int != 42 {
		t.Fatalf("expected 42, got %d", v.List[1].Int)
	}
}

func TestDecodeEmptyList(t *testing.T) {
	v, err := Decode([]byte("le"))
	if err != nil {
		t.Fatal(err)
	}
	if len(v.List) != 0 {
		t.Fatalf("expected empty list, got %d elements", len(v.List))
	}
}

func TestDecodeDict(t *testing.T) {
	v, err := Decode([]byte("d3:bar4:spam3:fooi42ee"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Type != Dict {
		t.Fatalf("expected dict, got %s", v.Type)
	}
	if v.Dict["bar"].Str != "spam" {
		t.Fatalf("expected bar=spam, got bar=%q", v.Dict["bar"].Str)
	}
	if v.Dict["foo"].Int != 42 {
		t.Fatalf("expected foo=42, got foo=%d", v.Dict["foo"].Int)
	}
}

func TestDecodeEmptyDict(t *testing.T) {
	v, err := Decode([]byte("de"))
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Dict) != 0 {
		t.Fatalf("expected empty dict, got %d entries", len(v.Dict))
	}
}

func TestDecodeNested(t *testing.T) {
	v, err := Decode([]byte("d4:listl5:item1i2eee"))
	if err != nil {
		t.Fatal(err)
	}
	list := v.Dict["list"].List
	if list[0].Str != "item1" {
		t.Fatalf("expected 'item1', got %q", list[0].Str)
	}
	if list[1].Int != 2 {
		t.Fatalf("expected 2, got %d", list[1].Int)
	}
}

func TestDecodeTrailingData(t *testing.T) {
	_, err := Decode([]byte("i1exyz"))
	if err == nil {
		t.Fatal("expected error for trailing data")
	}
}

func TestDecodeTruncatedString(t *testing.T) {
	_, err := Decode([]byte("5:hi"))
	if err == nil {
		t.Fatal("expected error for truncated string")
	}
}

func TestEncodeString(t *testing.T) {
	v := Value{Type: String, Str: "spam"}
	if string(v.Encode()) != "4:spam" {
		t.Fatalf("expected '4:spam', got %q", string(v.Encode()))
	}
}

func TestEncodeInt(t *testing.T) {
	v := Value{Type: Int, Int: 42}
	if string(v.Encode()) != "i42e" {
		t.Fatalf("expected 'i42e', got %q", string(v.Encode()))
	}
}

func TestEncodeList(t *testing.T) {
	v := Value{
		Type: List,
		List: []Value{
			{Type: String, Str: "spam"},
			{Type: Int, Int: 42},
		},
	}
	if string(v.Encode()) != "l4:spami42ee" {
		t.Fatalf("expected 'l4:spami42ee', got %q", string(v.Encode()))
	}
}

func TestEncodeDict(t *testing.T) {
	v := Value{
		Type: Dict,
		Dict: map[string]Value{
			"bar": {Type: String, Str: "spam"},
			"foo": {Type: Int, Int: 42},
		},
	}
	encoded := string(v.Encode())
	if encoded != "d3:bar4:spam3:fooi42ee" {
		t.Fatalf("expected 'd3:bar4:spam3:fooi42ee', got %q", encoded)
	}
}

func TestRoundTrip(t *testing.T) {
	input := "d3:bar4:spam3:fooi42e4:listl4:testi123eee"
	v, err := Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	output := string(v.Encode())
	if input != output {
		t.Fatalf("round-trip failed:\n  input:  %q\n  output: %q", input, output)
	}
}

func TestDecodeLargeInt(t *testing.T) {
	v, err := Decode([]byte("i1234567890e"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 1234567890 {
		t.Fatalf("expected 1234567890, got %d", v.Int)
	}
}

func TestAsString(t *testing.T) {
	v := Value{Type: String, Str: "hello"}
	s, err := v.AsString()
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello" {
		t.Fatalf("expected 'hello', got %q", s)
	}
	_, err = Value{Type: Int}.AsString()
	if err == nil {
		t.Fatal("expected error when calling AsString on int")
	}
}

func TestAsInt(t *testing.T) {
	v := Value{Type: Int, Int: 99}
	n, err := v.AsInt()
	if err != nil {
		t.Fatal(err)
	}
	if n != 99 {
		t.Fatalf("expected 99, got %d", n)
	}
	_, err = Value{Type: String}.AsInt()
	if err == nil {
		t.Fatal("expected error when calling AsInt on string")
	}
}
