package connection

import (
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	"github.com/WhiCu/p2pFileShare/peer/message"
)

const (
	HeartbeatTimer    = 10 * time.Second // Время между пингами
	HeartbeatDeadline = 5 * time.Second
)

type Connection struct {
	Conn       net.Conn          // Соединение
	LastActive time.Time         // Время последней активности
	Username   string            // Имя пользователя
	Save       bool              // Сохранять ли историю чата
	Chat       []message.Message // История чата

	closeOnce sync.Once     // Гарантирует, что соединение закрывается только один раз
	closed    chan struct{} // Канал для закрытия соединения
}

// NewConnection создаёт новое соединение и запускает heartbeat для проверки активности
func NewConnection(conn net.Conn) *Connection {
	c := &Connection{
		Conn:       conn,
		LastActive: time.Now(),
		Save:       true,
		closed:     make(chan struct{}),
	}
	go c.Heartbeat()
	return c
}

// Heartbeat отправляет "ping" каждые HeartbeatTimer секунд, чтобы проверить активность узла
func (c *Connection) Heartbeat() {
	ticker := time.NewTicker(HeartbeatTimer)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			message := message.Message{
				Type:   "heartbeat",
				Sender: c.Addr(),
			}

			msgBytes, _ := json.Marshal(message)
			// Устанавливаем таймаут на отправку пинга
			if err := c.Conn.SetWriteDeadline(time.Now().Add(HeartbeatDeadline)); err != nil {
				log.Printf("Ошибка установки дедлайна записи для узла %s: %v", c.Conn.RemoteAddr().String(), err)
				c.Close()
				return
			}

			_, err := c.Conn.Write(append(msgBytes, '\n'))
			if err != nil {
				log.Printf("Ошибка при отправке пинга узлу %s: %v", c.Conn.RemoteAddr().String(), err)
				c.Close()
				return
			}

			c.LastActive = time.Now()

		case <-c.closed:
			log.Printf("Heartbeat остановлен для узла %s", c.Conn.RemoteAddr().String())
			return
		}
	}
}

// SetUsername задаёт имя пользователя для соединения
func (c *Connection) SetUsername(username string) {
	c.Username = username
}

// Close закрывает соединение и останавливает Heartbeat
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		log.Printf("Закрытие соединения с узлом %s", c.Conn.RemoteAddr().String())
		close(c.closed) // Закрываем канал для остановки Heartbeat
		if err := c.Conn.Close(); err != nil {
			log.Printf("Ошибка при закрытии соединения с узлом %s: %v", c.Conn.RemoteAddr().String(), err)
		}
	})
}

func (c *Connection) Addr() string {
	return c.Conn.RemoteAddr().String()
}
