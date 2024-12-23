package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/WhiCu/p2pFileShare/peer"
	"github.com/WhiCu/p2pFileShare/peer/connection"
)

func main() {

	port := "8080"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	p := peer.NewTCPPeer("testPeer", "localhost", port)

	//p.StartBootstrap(config.MustGet("BOOTSTRAP_PORT"))
	go p.StartTCPListener()

	if len(os.Args) > 2 {
		p.ConnectToPeers(os.Args[2:]...)
	}

	go waitForExit()

	consoleReader := bufio.NewReader(os.Stdin)
	for {
		var text string

		text += fmt.Sprintf("Текущее имя узла: %s\n", p.Username)
		text += fmt.Sprintf("Текущий ip узла: %s\n", p.Addr())

		text += "Log:\n"

		for _, msg := range p.Log() {
			text += fmt.Sprintf("%s: %s\n", msg.Sender, msg.Content)
		}

		text += "Текущие подключения:\n"

		p.Connections.Range(func(key, value any) bool {
			text += fmt.Sprintf("%s", key)
			if value.(*connection.Connection).Username != "" {
				text += " (" + value.(*connection.Connection).Username + ")\n"
			}
			text += "\n"
			return true
		})

		text += "Текущая история:\n"

		p.Connections.Range(func(key, value any) bool {
			for _, msg := range value.(*connection.Connection).Chat {
				text += fmt.Sprintf("%s: %s\n", msg.Sender, msg.Content)
			}
			return true
		})

		text += "Ваше Сообщение\n>"
		time.Sleep(100 * time.Millisecond)
		fmt.Println(text)
		message, _ := consoleReader.ReadString('\n')
		message = strings.TrimSpace(message)

		if message == "exit" {
			fmt.Println("Выход...")
			break
		}

		p.SendMessageToPeers(message)

		// if strings.HasPrefix(text, "file ") {
		// 	filePath := strings.TrimPrefix(text, "file ")
		// 	p.sendFileToPeers(filePath)
		// } else {
		// 	p.sendMessageToPeers(text)
		// }
	}
}

func waitForExit() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan
	log.Println("Завершение программы...")
	time.Sleep(2 * time.Second) // Задержка для корректного завершения всех горутин
	os.Exit(0)
}
