package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net"
	"net/url"
)

type Message struct {
	RND string
	ORG string
}

var message = &Message{
	RND: "",
	ORG: "",
}

func main() {
	go makeConnection("20241", "7001")
	go makeConnection("20242", "7002")
	go makeConnection("3309", "7009")

	select {}
}

func makeConnection(local_port string, remote_port string) {
	for {
		// Listen for incoming connections from admin system
		listener, err := net.Listen("tcp", "localhost:"+local_port)
		if err != nil {
			log.Fatalf("Failed to start listener: %v", err)
		}

		// Accept connections from counters and proxy theme
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			return
		}

		// Establish WebSocket connection to VPS
		vpsURL := url.URL{Scheme: "ws", Host: "192.168.1.104", Path: "/counter"}
		query := vpsURL.Query()
		query.Set("port", remote_port)
		vpsURL.RawQuery = query.Encode()

		ws, _, err := websocket.DefaultDialer.Dial(vpsURL.String(), nil)
		if err != nil {
			log.Fatalf("Failed to connect to VPS: %v", err)
		}

		//read RND and ORG from socket and send it  to server
		buffer := make([]byte, 512)
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("TCP connection closed: %v", err)
			return
		}

		if message.RND == "" {
			if err := json.Unmarshal(buffer[:n], &message); err != nil {
				log.Printf("Error decode json: %v", err)
				return
			}
		}

		//send first for server and wait for response from server
		err = ws.WriteMessage(websocket.TextMessage, buffer[:n])
		if err != nil {
			log.Fatalf("Failed to send KEY: %v", err)
		}
		//send second time for admin
		err = ws.WriteMessage(websocket.TextMessage, buffer[:n])
		if err != nil {
			log.Fatalf("Failed to send KEY: %v", err)
		}

		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Printf("WebSocket connection closed: %v", err)
			return
		}

		fmt.Println(string(message))

		go handleConnection(conn, ws)
	}
}

func handleConnection(tcpConn net.Conn, ws *websocket.Conn) {
	defer tcpConn.Close()
	defer ws.Close()

	// Goroutine to forward data from TCP to WebSocket
	go func() {
		buffer := make([]byte, 32*1024)
		for {
			// Read from TCP connection
			n, err := tcpConn.Read(buffer)
			if err != nil {
				log.Printf("TCP connection closed: %v", err)
				return
			}

			// Forward to WebSocket
			err = ws.WriteMessage(websocket.BinaryMessage, buffer[:n])
			if err != nil {
				log.Printf("Failed to send data to WebSocket: %v", err)
				return
			}
		}
	}()

	// Forward data from WebSocket to TCP
	for {
		// Read from WebSocket
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Printf("WebSocket connection closed: %v", err)
			return
		}

		// Write to TCP connection
		_, err = tcpConn.Write(message)
		if err != nil {
			log.Printf("Failed to write data to TCP: %v", err)
			return
		}
	}
}
