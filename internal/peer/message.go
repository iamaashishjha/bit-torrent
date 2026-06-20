package peer

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	Choke         byte = 0
	Unchoke       byte = 1
	Interested    byte = 2
	NotInterested byte = 3
	Have          byte = 4
	Bitfield      byte = 5
	Request       byte = 6
	Piece         byte = 7
	Cancel        byte = 8
)

type Message struct {
	ID      byte
	Payload []byte
}

func (m *Message) Serialize() []byte {
	if m == nil {
		length := make([]byte, 4)
		binary.BigEndian.PutUint32(length, 0)
		return length
	}

	length := uint32(1 + len(m.Payload))
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = m.ID
	copy(buf[5:], m.Payload)
	return buf
}

func ReadMessage(r io.Reader) (*Message, error) {
	lenBuf := make([]byte, 4)
	_, err := io.ReadFull(r, lenBuf)
	if err != nil {
		return nil, fmt.Errorf("message: reading length: %v", err)
	}

	length := binary.BigEndian.Uint32(lenBuf)

	if length == 0 {
		return nil, nil
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, fmt.Errorf("message: reading body: %v", err)
	}

	return &Message{
		ID:      buf[0],
		Payload: buf[1:],
	}, nil
}

func WriteMessage(w io.Writer, msg *Message) error {
	data := msg.Serialize()
	_, err := w.Write(data)
	if err != nil {
		return fmt.Errorf("message: write: %v", err)
	}
	return nil
}

func ParseHave(msg *Message) (uint32, error) {
	if msg.ID != Have {
		return 0, fmt.Errorf("expected have message, got %d", msg.ID)
	}
	if len(msg.Payload) < 4 {
		return 0, fmt.Errorf("have message too short")
	}
	return binary.BigEndian.Uint32(msg.Payload[0:4]), nil
}

func ParseRequest(msg *Message) (index, begin, length uint32, err error) {
	if msg.ID != Request {
		err = fmt.Errorf("expected request message, got %d", msg.ID)
		return
	}
	if len(msg.Payload) < 12 {
		err = fmt.Errorf("request message too short")
		return
	}
	index = binary.BigEndian.Uint32(msg.Payload[0:4])
	begin = binary.BigEndian.Uint32(msg.Payload[4:8])
	length = binary.BigEndian.Uint32(msg.Payload[8:12])
	return
}

func ParsePiece(msg *Message) (index, begin uint32, data []byte, err error) {
	if msg.ID != Piece {
		err = fmt.Errorf("expected piece message, got %d", msg.ID)
		return
	}
	if len(msg.Payload) < 8 {
		err = fmt.Errorf("piece message too short")
		return
	}
	index = binary.BigEndian.Uint32(msg.Payload[0:4])
	begin = binary.BigEndian.Uint32(msg.Payload[4:8])
	data = msg.Payload[8:]
	return
}

func NewRequest(index, begin, length uint32) *Message {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], begin)
	binary.BigEndian.PutUint32(payload[8:12], length)
	return &Message{ID: Request, Payload: payload}
}

func NewInterested() *Message {
	return &Message{ID: Interested}
}

func NewUnchoke() *Message {
	return &Message{ID: Unchoke}
}

func NewChoke() *Message {
	return &Message{ID: Choke}
}

func NewHave(index uint32) *Message {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, index)
	return &Message{ID: Have, Payload: payload}
}
