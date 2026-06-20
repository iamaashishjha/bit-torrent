package bencode

import (
	"fmt"
	"sort"
)

type Type int

const (
	String Type = iota
	Int
	List
	Dict
)

type Value struct {
	Type  Type
	Str   string
	Int   int64
	List  []Value
	Dict  map[string]Value
}

func (v Value) AsString() (string, error) {
	if v.Type != String {
		return "", fmt.Errorf("bencode: value is %s, not string", v.Type)
	}
	return v.Str, nil
}

func (v Value) AsInt() (int64, error) {
	if v.Type != Int {
		return 0, fmt.Errorf("bencode: value is %s, not int", v.Type)
	}
	return v.Int, nil
}

func (v Value) AsList() ([]Value, error) {
	if v.Type != List {
		return nil, fmt.Errorf("bencode: value is %s, not list", v.Type)
	}
	return v.List, nil
}

func (v Value) AsDict() (map[string]Value, error) {
	if v.Type != Dict {
		return nil, fmt.Errorf("bencode: value is %s, not dict", v.Type)
	}
	return v.Dict, nil
}

func (v Value) Encode() []byte {
	switch v.Type {
	case String:
		s := fmt.Sprintf("%d:%s", len(v.Str), v.Str)
		return []byte(s)
	case Int:
		s := fmt.Sprintf("i%de", v.Int)
		return []byte(s)
	case List:
		b := []byte{'l'}
		for _, item := range v.List {
			b = append(b, item.Encode()...)
		}
		b = append(b, 'e')
		return b
	case Dict:
		b := []byte{'d'}
		keys := make([]string, 0, len(v.Dict))
		for k := range v.Dict {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			kv := Value{Type: String, Str: k}
			b = append(b, kv.Encode()...)
			b = append(b, v.Dict[k].Encode()...)
		}
		b = append(b, 'e')
		return b
	default:
		return nil
	}
}

func (t Type) String() string {
	switch t {
	case String:
		return "string"
	case Int:
		return "int"
	case List:
		return "list"
	case Dict:
		return "dict"
	default:
		return "unknown"
	}
}
