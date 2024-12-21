package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/WhiCu/p2pFileShare/config"
	"github.com/WhiCu/p2pFileShare/peer"
)

func main() {

	port := config.MustGet("PORT_PEER")
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	p := peer.NewTCPPeer(config.MustGet("HOST_PEER"), port)

	//p.StartBootstrap(config.MustGet("BOOTSTRAP_PORT"))
	go p.StartTCPListener()

	if len(os.Args) > 2 {
		p.ConnectToPeer(os.Args[2])
	}

	go waitForExit()

	consoleReader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		text, _ := consoleReader.ReadString('\n')
		text = strings.TrimSpace(text)

		if text == "exit" {
			fmt.Println("Выход...")
			break
		}

		p.SendMessageToPeers(text)

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
