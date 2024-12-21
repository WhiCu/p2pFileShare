package message

import (
	"encoding/json"
	"errors"
)

// Сообщение, которое отправляется между узлами
type Message struct {
	Type     string `json:"type"` // Тип сообщения (text/file)
	Sender   string `json:"sender"`
	Content  string `json:"content"`
	Filename string `json:"filename,omitempty"`
}

func NewMessage(typeFormat string, sender string, content string, filename *string) (*Message, error) {
	switch typeFormat {
	case "text":
		return &Message{Type: "text", Sender: sender, Content: content}, nil
	case "file":
		if filename == nil {
			return nil, errors.New("filename is nil")
		}
		return &Message{Type: "file", Sender: sender, Content: content, Filename: *filename}, nil
	}
	return nil, errors.New("unknown type")
}

func MessageFromString(line string) (*Message, error) {
	return MessageFromBytes([]byte(line))
}
func MessageFromBytes(bytes []byte) (*Message, error) {
	var m Message
	err := json.Unmarshal(bytes, &m)
	return &m, err
}

func (m *Message) Bytes() (string, error) {
	bytes, err := json.Marshal(m)
	return string(bytes), err
}
