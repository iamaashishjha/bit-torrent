package bencode

import (
	"fmt"
	"strconv"
)

func Decode(data []byte) (Value, error) {
	i := 0
	v, err := decodeValue(data, &i)
	if err != nil {
		return Value{}, err
	}
	if i != len(data) {
		return Value{}, fmt.Errorf("bencode: unexpected trailing byte at position %d", i)
	}
	return v, nil
}

func decodeValue(data []byte, i *int) (Value, error) {
	if *i >= len(data) {
		return Value{}, fmt.Errorf("bencode: unexpected end of data")
	}
	switch {
	case data[*i] >= '0' && data[*i] <= '9':
		return decodeString(data, i)
	case data[*i] == 'i':
		return decodeInt(data, i)
	case data[*i] == 'l':
		return decodeList(data, i)
	case data[*i] == 'd':
		return decodeDict(data, i)
	default:
		return Value{}, fmt.Errorf("bencode: unexpected byte %q at position %d", data[*i], *i)
	}
}

func decodeString(data []byte, i *int) (Value, error) {
	colon := *i
	for colon < len(data) && data[colon] != ':' {
		if data[colon] < '0' || data[colon] > '9' {
			return Value{}, fmt.Errorf("bencode: invalid character in string length at position %d", colon)
		}
		colon++
	}
	if colon >= len(data) {
		return Value{}, fmt.Errorf("bencode: unterminated string length")
	}
	length, err := strconv.Atoi(string(data[*i:colon]))
	if err != nil {
		return Value{}, fmt.Errorf("bencode: invalid string length: %v", err)
	}
	if length < 0 {
		return Value{}, fmt.Errorf("bencode: negative string length")
	}
	*i = colon + 1
	if *i+length > len(data) {
		return Value{}, fmt.Errorf("bencode: string length %d exceeds remaining data (%d bytes)", length, len(data)-*i)
	}
	s := string(data[*i : *i+length])
	*i += length
	return Value{Type: String, Str: s}, nil
}

func decodeInt(data []byte, i *int) (Value, error) {
	*i++ // skip 'i'
	start := *i
	for *i < len(data) && data[*i] != 'e' {
		*i++
	}
	if *i >= len(data) {
		return Value{}, fmt.Errorf("bencode: unterminated integer")
	}
	n, err := strconv.ParseInt(string(data[start:*i]), 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("bencode: invalid integer: %v", err)
	}
	*i++ // skip 'e'
	return Value{Type: Int, Int: n}, nil
}

func decodeList(data []byte, i *int) (Value, error) {
	*i++ // skip 'l'
	var list []Value
	for *i < len(data) && data[*i] != 'e' {
		v, err := decodeValue(data, i)
		if err != nil {
			return Value{}, err
		}
		list = append(list, v)
	}
	if *i >= len(data) {
		return Value{}, fmt.Errorf("bencode: unterminated list")
	}
	*i++ // skip 'e'
	return Value{Type: List, List: list}, nil
}

func decodeDict(data []byte, i *int) (Value, error) {
	*i++ // skip 'd'
	dict := make(map[string]Value)
	for *i < len(data) && data[*i] != 'e' {
		key, err := decodeValue(data, i)
		if err != nil {
			return Value{}, err
		}
		if key.Type != String {
			return Value{}, fmt.Errorf("bencode: dict key must be a string")
		}
		val, err := decodeValue(data, i)
		if err != nil {
			return Value{}, err
		}
		dict[key.Str] = val
	}
	if *i >= len(data) {
		return Value{}, fmt.Errorf("bencode: unterminated dict")
	}
	*i++ // skip 'e'
	return Value{Type: Dict, Dict: dict}, nil
}
