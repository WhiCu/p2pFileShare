package peer

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/WhiCu/p2pFileShare/peer/bootstrap"
	"github.com/WhiCu/p2pFileShare/peer/connection"
	"github.com/WhiCu/p2pFileShare/peer/message"
)

const (
	connReadDeadline = 20 * time.Minute // Время ожидания чтения соединения
)

// Peer представляет узел в P2P-сети
// Он содержит информацию о локальном хосте, порте и активных подключениях
// Также управляет прослушиванием входящих соединений

type Peer struct {
	Username    string       // Имя пользователя
	Host        string       // Хост узла (IP-адрес)
	Port        string       // Порт, на котором работает узел
	Connections *sync.Map    // Список активных соединений с другими узлами
	Listener    net.Listener // Слушатель для входящих TCP-соединений

	BootstrapPort string                     // Порт, на котором работает Bootstrap-сервер
	Bootstrap     *bootstrap.BootstrapServer // Сервер для приёма входящих соединений
}

// NewTCPPeer создаёт новый узел (Peer) с указанным хостом и портом
func NewTCPPeer(host string, port string, username string) *Peer {
	return &Peer{
		Username:    username,
		Host:        host,
		Port:        port,
		Connections: &sync.Map{},
		Bootstrap:   nil,
	}
}

// Addr возвращает полный адрес узла в формате "host:port" или "[host]:port"
func (p *Peer) Addr() string {
	return net.JoinHostPort(p.Host, p.Port)
}

// TCPAddr возвращает TCP-адрес узла, который можно использовать для подключения или прослушивания
func (p *Peer) TCPAddr() *net.TCPAddr {
	address, err := net.ResolveTCPAddr("tcp", p.Addr())
	if err != nil {
		log.Fatalf("%s.TCPAddr: не удалось разрешить TCP-адрес: %v", p.Addr(), err)
	}
	return address
}

func (p *Peer) StartBootstrap(bootstrapPort string) {
	p.BootstrapPort = bootstrapPort
	p.Bootstrap = bootstrap.NewBootstrapServer(p.Host, bootstrapPort, p.Connections)
	p.Bootstrap.Start()
}

// StartTCPListener запускает TCP-сервер, который принимает входящие соединения
// Входящие подключения обрабатываются в отдельной горутине
func (p *Peer) StartTCPListener() {
	listener, err := net.ListenTCP("tcp", p.TCPAddr())
	if err != nil {
		log.Fatalf("%s.StartTCPListener: не удалось запустить TCP-сервер: %v", p.Addr(), err)
	}
	p.Listener = listener
	log.Printf("TCP-сервер запущен на %s", p.Addr())
	go p.acceptIncomingConnections()
}

// ConnectToPeer устанавливает соединение с удалённым узлом по адресу
// Если соединение не удаётся, выполняются повторные попытки каждые 5 секунд
func (p *Peer) ConnectToPeer(address string) {
	for {
		if _, exists := p.Connections.Load(address); exists {
			return // Уже подключены к этому узлу
		}
		conn, err := net.Dial("tcp", address)
		if err != nil {
			log.Printf("%s.ConnectToPeer: не удалось подключиться к узлу %s: %v", p.Addr(), address, err)
			log.Printf("Повторное подключение к %s через 5 секунд...", address)
			time.Sleep(5 * time.Second)
			continue
		}
		p.registerConnection(address, conn)
		break
	}
}

// acceptIncomingConnections принимает входящие соединения от других узлов
// Каждое входящее соединение обрабатывается в отдельной горутине
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

// registerConnection регистрирует новое соединение с удалённым узлом
// Подключение добавляется в список активных соединений, а приём сообщений с узла обрабатывается в отдельной горутине
func (p *Peer) registerConnection(address string, conn net.Conn) {
	info := message.Message{
		Type:   "info",
		Sender: p.Username,
	}
	c := connection.NewConnection(conn, info)
	p.Connections.Store(address, c)
	log.Printf("Подключение к узлу %s успешно установлено", address)
	go p.handleConnection(c)
}

// handleConnection обрабатывает соединение с удалённым узлом
// Считывает сообщения от узла и выводит их в лог
// В случае закрытия соединения оно удаляется из списка активных соединений
func (p *Peer) handleConnection(conn *connection.Connection) {
	defer func() {
		conn.Close()
		p.Connections.Delete(conn.Addr())
		log.Printf("Соединение с %s закрыто", conn.Addr())
	}()

	scanner := bufio.NewScanner(conn.Conn)
	for {
		// Устанавливаем дедлайн на чтение
		conn.Conn.SetReadDeadline(time.Now().Add(connReadDeadline))

		if !scanner.Scan() {
			// EOF или дедлайн завершает соединение
			break
		}

		line := scanner.Text()

		msg, err := message.MessageFromString(line)
		if err != nil {
			fmt.Printf("Ошибка при разборе сообщения: %s\n", err)
			continue
		}

		switch msg.Type {
		case "info":
			conn.Username = msg.Content
		case "heartbeat":
			continue
		case "text":
			fmt.Printf("[От %s]: %s\n", msg.Sender, msg.Content)
		case "file":
			fmt.Printf("[От %s]: Получен файл %s\n", msg.Sender, msg.Filename)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("%s.handleConnection: ошибка при чтении из соединения: %v", conn.Addr(), err)
	}
}

func (p *Peer) SendMessageToPeers(text string) {
	var wg sync.WaitGroup
	p.Connections.Range(func(key, value any) bool {
		wg.Add(1)
		go p.SendMessageToPeer(value.(*connection.Connection), text, &wg)
		return true
	})
	wg.Wait()
}

// SendMessageToPeer отправляет текстовое сообщение на соединение с узлом
// Если wg != nil, то wg.Done() будет вызван после отправки сообщения
func (p *Peer) SendMessageToPeer(conn *connection.Connection, text string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	msg := message.Message{
		Type:    "text",
		Sender:  p.Addr(),
		Content: text,
	}

	err := conn.Send(msg)
	if err != nil {
		log.Printf("Ошибка при отправке сообщения узлу %s: %v", conn.Addr(), err)
	}
}
