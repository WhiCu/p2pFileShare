package bootstrap

import (
	"fmt"
	"log"
	"net"
	"sync"
)

type BootstrapServer struct {
	Host  string
	Port  string
	Peers *sync.Map
}

func NewBootstrapServer(host string, port string, peerMap *sync.Map) *BootstrapServer {
	return &BootstrapServer{
		Host:  host,
		Port:  port,
		Peers: peerMap,
	}
}

// Addr возвращает полный адрес узла в формате "host:port" или "[host]:port"
func (p *BootstrapServer) Addr() string {
	return net.JoinHostPort(p.Host, p.Port)
}

// TCPAddr возвращает TCP-адрес узла, который можно использовать для подключения или прослушивания
func (p *BootstrapServer) TCPAddr() *net.TCPAddr {
	address, err := net.ResolveTCPAddr("tcp", p.Addr())
	if err != nil {
		log.Fatalf("%s.TCPAddr: не удалось разрешить TCP-адрес: %v", p.Addr(), err)
	}
	return address
}

// Start запускает сервер для приёма входящих соединений
func (b *BootstrapServer) Start() {
	listener, err := net.ListenTCP("tcp", b.TCPAddr())
	if err != nil {
		log.Fatalf("Ошибка при запуске Bootstrap-сервера: %v", err)
	}
	log.Printf("Bootstrap-сервер запущен на порту %s", b.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Ошибка при приёме соединения: %v", err)
			continue
		}
		go b.handleConnection(conn)
	}
}

// handleConnection обрабатывает подключение нового узла
func (b *BootstrapServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("Новое подключение от %s", remoteAddr)

	if _, exists := b.Peers.Load(remoteAddr); exists {
		log.Printf("Узел %s уже зарегистрирован", remoteAddr) // Как так вообще? Уже должен был подключен к данному Peer
		return
	}
	b.sendPeerList(conn)
}

// sendPeerList отправляет список всех известных узлов клиенту
func (b *BootstrapServer) sendPeerList(conn net.Conn) {

	b.Peers.Range(func(key, value interface{}) bool {
		if _, err := fmt.Fprintln(conn, key); err != nil {
			log.Printf("Ошибка при отправке списка узлов: %v", err)
		}
		return true
	})
}
