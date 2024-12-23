// Пакет connection управляет соединениями между узлами в P2P-сети.
// Он включает управление активностью соединений, отправку и приём сообщений.
package connection

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/WhiCu/p2pFileShare/peer/message"
)

const (
	// HeartbeatTimer - интервал между отправками пинга
	HeartbeatTimer = 10 * time.Second
	// HeartbeatDeadline - время ожидания отклика перед закрытием соединения
	HeartbeatDeadline = 15 * time.Second
)

// Connection представляет соединение между узлами P2P-сети.
type Connection struct {
	Conn       net.Conn          // Низкоуровневое сетевое соединение
	LastActive time.Time         // Время последней активности
	Username   string            // Имя пользователя узла
	Save       bool              // Флаг сохранения истории чата
	Chat       []message.Message // История чата

	closeOnce sync.Once     // Гарантирует, что соединение закрывается один раз
	closed    chan struct{} // Канал для завершения работы соединения
	IsClosed  bool          // Флаг, указывающий, что соединение закрыто
}

// NewConnection создаёт новое соединение и запускает Heartbeat для проверки активности.
func NewConnection(conn net.Conn, info message.Message) *Connection {
	c := &Connection{
		Username:   conn.RemoteAddr().String(),
		Conn:       conn,
		LastActive: time.Now(),
		Save:       true,
		Chat:       make([]message.Message, 0),
		closed:     make(chan struct{}),
		IsClosed:   false,
	}
	if !c.sendInfo(info) {
		log.Printf("%s.NewConnection: не удалось отправить информацию о соединении", c.Addr())
		c.Close()
	}
	go c.heartbeat()
	return c
}

// sendInfo отправляет сообщение о соединении.
func (c *Connection) sendInfo(messageInfo message.Message) bool {
	data, err := messageInfo.Bytes()
	if err != nil {
		log.Printf("%s.sendInfo: не удалось преобразовать сообщение в строку: %v", c.Addr(), err)
		return false
	}
	_, err = c.Conn.Write([]byte(data + "\n"))
	if err != nil {
		log.Printf("%s.sendInfo: не удалось отправить сообщение: %v", c.Addr(), err)
		return false
	}
	return true
}

// Send отправляет сообщение на удалённый узел.
func (c *Connection) Send(msg message.Message) error {
	data, err := msg.Bytes()
	if err != nil {
		log.Printf("%s.Send: не удалось преобразовать сообщение в строку: %v", c.Addr(), err)
		return errors.New("не удалось преобразовать сообщение в строку")
	}
	_, err = c.Conn.Write([]byte(data + "\n"))
	if err != nil {
		log.Printf("%s.Send: не удалось отправить сообщение: %v", c.Addr(), err)
		c.Close()
		return errors.New("не удалось отправить сообщение")
	}
	if msg.Type == "text" && c.Save {
		c.AddMessages(msg)
	}
	return nil
}

// heartbeat отправляет сообщение каждые HeartbeatTimer секунд для проверки активности узла.
func (c *Connection) heartbeat() {
	ticker := time.NewTicker(HeartbeatTimer)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			msg := message.Message{
				Type:   "heartbeat",
				Sender: c.Addr(),
			}

			// Устанавливаем таймаут на отправку пинга
			if err := c.Conn.SetWriteDeadline(time.Now().Add(HeartbeatDeadline)); err != nil {
				log.Printf("Ошибка установки дедлайна записи для узла %s: %v", c.Addr(), err)
				c.Close()
				return
			}

			if err := c.Send(msg); err != nil {
				log.Printf("Ошибка при отправке пинга узлу %s: %v", c.Addr(), err)
				c.Close()
				return
			}

			c.LastActive = time.Now()

		case <-c.closed:
			log.Printf("Heartbeat остановлен для узла %s", c.Addr())
			return
		}
	}
}

// SetUsername устанавливает имя пользователя для соединения.
func (c *Connection) SetUsername(username string) {
	c.Username = username
}

// Close завершает соединение и останавливает Heartbeat.
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		log.Printf("Закрытие соединения с узлом %s", c.Addr())
		close(c.closed) // Закрываем канал для остановки Heartbeat
		if err := c.Conn.Close(); err != nil {
			log.Printf("Ошибка при закрытии соединения с узлом %s: %v", c.Addr(), err)
		}
		c.IsClosed = true
	})
}

// Addr возвращает адрес удалённого узла в формате "host:port".
func (c *Connection) Addr() string {
	return c.Conn.RemoteAddr().String()
}

func (c *Connection) AddMessages(messages ...message.Message) {
	c.Chat = append(c.Chat, messages...)
}
