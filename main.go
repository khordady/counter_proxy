package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
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

// var server = "192.168.1.105:9090"
var server = "expanel.app"
var address = "/websocket"

// var wss = "ws"
var wss = "wss"

func main() {
	go makeConnection("20241")
	go makeConnection("20242")
	makeConnection("3309")
}

func makeConnection(local_port string) {
	// Listen for incoming connections from admin system
	listener, err := net.Listen("tcp", "localhost:"+local_port)
	if err != nil {
		log.Printf("Failed to start listener: %v", err)
	}

	fmt.Println("Start listening:", local_port)

	for {
		// Accept connections from counters and proxy theme
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		fmt.Println("Socket Accepted:", local_port)

		// Establish WebSocket connection to VPS
		vpsURL := url.URL{Scheme: wss, Host: server, Path: address + "/counter"}
		query := vpsURL.Query()
		query.Set("port", local_port)
		vpsURL.RawQuery = query.Encode()

		ws, _, err := websocket.DefaultDialer.Dial(vpsURL.String(), nil)
		if err != nil || ws == nil {
			log.Printf("Failed to connect to VPS: %v", err)
			conn.Close()
			continue
		}

		fmt.Println("WS connected:", local_port)

		if local_port == "3309" {
			err = ws.WriteJSON(message)
			if err != nil {
				log.Printf("Failed to send DB message: %v", err)
				ws.Close()
				conn.Close()
				continue
			}
			fmt.Println("Sent First message DB")

			err = ws.WriteJSON(message)
			if err != nil {
				log.Printf("Failed to send DB message: %v", err)
				ws.Close()
				conn.Close()
				continue
			}
			fmt.Println("Sent Second message DB")

		} else {
			lengthBytes, messageBytes := readFirstMessage(conn)

			if lengthBytes == nil || messageBytes == nil {
				ws.Close()
				conn.Close()
				continue
			}

			success := repeatMessage(ws, lengthBytes, messageBytes)

			if !success {
				ws.Close()
				conn.Close()
				continue
			}

			//send second time for admin
			success = repeatMessage(ws, lengthBytes, messageBytes)
			if !success {
				ws.Close()
				conn.Close()
				continue
			}
		}

		go handleConnection(conn, ws)
	}
}

func readFirstMessage(conn net.Conn) ([]byte, []byte) {
	//read RND and ORG from socket and send it  to server
	//read byte size first
	lengthBytes := make([]byte, 4)
	_, err := io.ReadFull(conn, lengthBytes)
	if err != nil {
		if err == io.EOF {
			fmt.Println("Connection closed by client.")
		} else {
			fmt.Println("Error reading length:", err)
		}
		conn.Close()
		return nil, nil
	}

	// Convert length bytes to an integer
	var messageLength int32
	buffer := bytes.NewBuffer(lengthBytes)
	err = binary.Read(buffer, binary.BigEndian, &messageLength)
	if err != nil {
		fmt.Println("Error decoding length:", err)
		return nil, nil
	}

	// Step 2: Read the message content in a loop (if necessary)
	messageBytes := make([]byte, messageLength)
	_, err = io.ReadFull(conn, messageBytes) // Ensures the full message is read
	if err != nil {
		fmt.Println("Error reading message:", err)
		return nil, nil
	}

	if message.RND == "" {
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			log.Printf("Error decode json: %v", err)
			return nil, nil
		}
	}

	return lengthBytes, messageBytes
}

func repeatMessage(ws *websocket.Conn, lengthBytes, messageBytes []byte) bool {
	err := ws.WriteMessage(websocket.BinaryMessage, lengthBytes)
	if err != nil {
		log.Printf("Failed to send LENGTH: %v", err)
		return false
	}

	err = ws.WriteMessage(websocket.TextMessage, messageBytes)
	if err != nil {
		log.Printf("Failed to send MESSAGE: %v", err)
		return false
	}

	return true
}

func handleConnection(tcpConn net.Conn, ws *websocket.Conn) {
	defer tcpConn.Close()
	defer ws.Close()

	// Goroutine to forward data from TCP to WebSocket
	go func() {
		defer tcpConn.Close()
		defer ws.Close()

		buffer := make([]byte, 32*1024)
		for {
			// Read from TCP connection
			n, err := tcpConn.Read(buffer)
			if n > 0 {
				// Forward to WebSocket
				err = ws.WriteMessage(websocket.BinaryMessage, buffer[:n])
				if err != nil {
					log.Printf("Failed to send data to WebSocket: %v", err)
					return
				}
			}
			if err != nil {
				log.Printf("TCP connection closed: %v", err)
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
