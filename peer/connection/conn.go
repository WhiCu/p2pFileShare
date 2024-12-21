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
	//!!! HeartbeatTimer < HeartbeatDeadline
	HeartbeatTimer    = 10 * time.Second // Время между пингами
	HeartbeatDeadline = 15 * time.Second
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
func NewConnection(conn net.Conn, info message.Message) *Connection {
	c := &Connection{
		Username:   info.Sender,
		Conn:       conn,
		LastActive: time.Now(),
		Save:       true,
		closed:     make(chan struct{}),
	}
	if !c.sendInfo(info) {
		log.Printf("%s.NewConnection: не удалось отправить информацию о соединении", c.Addr())
		c.Close()
	}
	go c.heartbeat()
	return c
}

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

func (c *Connection) Send(message message.Message) error {
	data, err := message.Bytes()
	if err != nil {
		log.Printf("%s.Send: не удалось преобразовать сообщение в строку: %v", c.Addr(), err)
		return errors.New("не удалось преобразовать сообщение в строку")
	}
	_, err = c.Conn.Write([]byte(data + "\n"))
	if err != nil {
		log.Printf("%s.Send: не удалось отправить сообщение: %v", c.Addr(), err)
		return errors.New("не удалось отправить сообщение")
	}
	return nil
}

// Heartbeat отправляет "ping" каждые HeartbeatTimer секунд, чтобы проверить активность узла
func (c *Connection) heartbeat() {
	ticker := time.NewTicker(HeartbeatTimer)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			message := message.Message{
				Type:   "heartbeat",
				Sender: c.Addr(),
			}
			// Устанавливаем таймаут на отправку пинга
			if err := c.Conn.SetWriteDeadline(time.Now().Add(HeartbeatDeadline)); err != nil {
				log.Printf("Ошибка установки дедлайна записи для узла %s: %v", c.Conn.RemoteAddr().String(), err)
				c.Close()
				return
			}

			err := c.Send(message)
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
