// Пакет peer реализует логику для узлов в P2P-сети.
// Он управляет активными соединениями, входящими соединениями и исходящей коммуникацией.
package peer

import (
	"bufio"
	"log"
	"net"
	"sync"
	"time"

	"github.com/WhiCu/p2pFileShare/peer/bootstrap"
	"github.com/WhiCu/p2pFileShare/peer/connection"
	"github.com/WhiCu/p2pFileShare/peer/message"
)

const (
	connReadDeadline = 20 * time.Minute // Таймаут для чтения из соединения
)

// Peer представляет узел в P2P-сети.
// Содержит информацию об адресе узла и активных соединениях.
// Также управляет прослушиванием входящих соединений.
type Peer struct {
	Username      string                     // Имя пользователя
	Host          string                     // Хост узла (IP-адрес)
	Port          string                     // Порт узла
	Connections   *sync.Map                  // Активные соединения с другими узлами
	Listener      net.Listener               // Слушатель для входящих TCP-соединений
	BootstrapPort string                     // Порт сервера Bootstrap
	Bootstrap     *bootstrap.BootstrapServer // Экземпляр Bootstrap-сервера
	log           []message.Message          // Журнал сообщений
}

// NewTCPPeer создаёт новый узел Peer с указанным именем пользователя, хостом и портом.
func NewTCPPeer(username, host, port string) *Peer {
	return &Peer{
		Username:    username,
		Host:        host,
		Port:        port,
		Connections: &sync.Map{},
	}
}

// Addr возвращает полный адрес узла в формате "host:port".
func (p *Peer) Addr() string {
	return net.JoinHostPort(p.Host, p.Port)
}

// TCPAddr разрешает и возвращает TCP-адрес узла.
func (p *Peer) TCPAddr() *net.TCPAddr {
	address, err := net.ResolveTCPAddr("tcp", p.Addr())
	if err != nil {
		log.Fatalf("%s.TCPAddr: не удалось разрешить TCP-адрес: %v", p.Addr(), err)
	}
	return address
}

// StartBootstrap инициализирует и запускает Bootstrap-сервер.
func (p *Peer) StartBootstrap(bootstrapPort string) {
	p.BootstrapPort = bootstrapPort
	p.Bootstrap = bootstrap.NewBootstrapServer(p.Host, bootstrapPort, p.Connections)
	p.Bootstrap.Start()
}

// StartTCPListener запускает TCP-сервер для принятия входящих соединений.
func (p *Peer) StartTCPListener() {
	listener, err := net.ListenTCP("tcp", p.TCPAddr())
	if err != nil {
		log.Fatalf("%s.StartTCPListener: не удалось запустить TCP-сервер: %v", p.Addr(), err)
	}
	p.Listener = listener
	log.Printf("TCP-сервер запущен на %s", p.Addr())
	go p.acceptIncomingConnections()
}

// ConnectToPeers устанавливает соединения с несколькими узлами.
// Для каждого адреса вызывается ConnectToPeer.
func (p *Peer) ConnectToPeers(addresses ...string) {
	for _, addr := range addresses {
		go p.ConnectToPeer(addr)
	}
}

// ConnectToPeer пытается подключиться к удалённому узлу по указанному адресу.
// Если соединение не удаётся, попытки повторяются каждые 5 секунд.
func (p *Peer) ConnectToPeer(address string) {
	for {
		if _, exists := p.Connections.Load(address); exists {
			return // Уже подключены к этому узлу
		}
		conn, err := net.Dial("tcp", address)
		if err != nil {
			log.Printf("%s.ConnectToPeer: не удалось подключиться к %s: %v", p.Addr(), address, err)
			log.Printf("Повторная попытка подключения к %s через 5 секунд...", address)
			time.Sleep(5 * time.Second)
			continue
		}
		p.registerConnection(address, conn)
		break
	}
}

// acceptIncomingConnections обрабатывает входящие соединения от других узлов.
// Каждое соединение обрабатывается в отдельной горутине.
func (p *Peer) acceptIncomingConnections() {
	for {
		conn, err := p.Listener.Accept()
		if err != nil {
			log.Printf("%s.acceptIncomingConnections: не удалось принять соединение: %v", p.Addr(), err)
			continue
		}
		log.Printf("Входящее соединение от %s", conn.RemoteAddr().String())
		p.registerConnection(conn.RemoteAddr().String(), conn)
	}
}

// registerConnection регистрирует новое соединение с удалённым узлом.
// Добавляет соединение в список активных и запускает горутину для его обработки.
func (p *Peer) registerConnection(address string, conn net.Conn) {
	info := message.Message{
		Type:   "info",
		Sender: p.Username,
	}
	c := connection.NewConnection(conn, info)
	p.Store(address, c)
	log.Printf("Подключение к узлу %s установлено", address)
	go p.handleConnection(c)
}

// handleConnection управляет взаимодействием с удалённым узлом.
// Читает сообщения от узла, логирует или обрабатывает их.
// При закрытии соединения оно удаляется из списка активных.
func (p *Peer) handleConnection(conn *connection.Connection) {
	defer func() {
		conn.Close()
		log.Printf("%s.handleConnection: Соединение с %s удалено", p.Addr(), conn.Addr())
		p.Connections.Delete(conn.Addr())
		log.Printf("%s.handleConnection: Соединение с %s закрыто", p.Addr(), conn.Addr())
	}()

	scanner := bufio.NewScanner(conn.Conn)
	for {
		// Устанавливаем дедлайн для предотвращения бесконечного ожидания
		conn.Conn.SetReadDeadline(time.Now().Add(connReadDeadline))

		if !scanner.Scan() {
			break // EOF или таймаут завершают соединение
		}

		line := scanner.Text()
		msg, err := message.MessageFromString(line)
		if err != nil {
			log.Printf("Ошибка при разборе сообщения: %v", err)
			continue
		}

		// Обработка разных типов сообщений
		switch msg.Type {
		case "info":
			log.Printf("%s: Информация о соединении: %s", conn.Addr(), msg.Sender)
			conn.SetUsername(msg.Sender)
		case "heartbeat":
			continue
		case "text":
			log.Printf("[От %s]: %s", msg.Sender, msg.Content)
			conn.AddMessages(*msg)
		case "log":
			p.log = append(p.log, *msg)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("%s.handleConnection: ошибка при чтении из соединения: %v", conn.Addr(), err)
	}
}

// SendMessageToPeers отправляет текстовое сообщение всем подключённым узлам.
func (p *Peer) SendMessageToPeers(text string) {
	var wg sync.WaitGroup
	p.Connections.Range(func(_, value any) bool {
		wg.Add(1)
		go p.SendMessageToPeer(value.(*connection.Connection), text, &wg)
		return true
	})
	wg.Wait()
}

// SendMessageToPeer отправляет текстовое сообщение конкретному узлу.
func (p *Peer) SendMessageToPeer(conn *connection.Connection, text string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	msg := message.Message{
		Type:    "text",
		Sender:  p.Addr(),
		Content: text,
	}

	if err := conn.Send(msg); err != nil {
		if conn.IsClosed {
			log.Printf("%s.SendMessageToPeer: Не удалось отправить сообщение узлу %s: закрыто", p.Addr(), conn.Addr())
			p.Connections.Range(func(key, value any) bool {
				log.Println(key, value.(*connection.Connection).Addr())
				return true
			})
		}
		log.Printf("%s.SendMessageToPeer: Не удалось отправить сообщение узлу %s: %v", p.Addr(), conn.Addr(), err)
	}
}

func (p *Peer) Log() []message.Message {
	return p.log
}

func (p *Peer) Store(key string, value *connection.Connection) {
	if host, port, _ := net.SplitHostPort(key); host == "localhost" {
		p.Connections.Store("127.0.0.1:"+port, value) // TODO: хардкод
		return
	}
	p.Connections.Store(key, value)
}
