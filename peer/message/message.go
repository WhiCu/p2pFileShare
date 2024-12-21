package message

// Сообщение, которое отправляется между узлами
type Message struct {
	Type     string `json:"type"` // Тип сообщения (text/file)
	Sender   string `json:"sender"`
	Content  string `json:"content"`
	Filename string `json:"filename,omitempty"`
}
