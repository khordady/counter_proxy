package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
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
	go connectServer("20240", "20241", "/co") //counter to officer
	go connectServer("20241", "20241", "/ca") //counter to admin
	go connectServer("20242", "20242", "/ca")
	connectServer("3310", "3309", "/ca")
}

func connectServer(local_port, remote_port, target string) {
	// Listen for incoming connections from admin system
	listener, err := net.Listen("tcp", "localhost:"+local_port)
	if err != nil {
		fmt.Printf("Failed to start listener: %v\n", err)
	}

	fmt.Println("Start listening:", local_port)

	for {
		// Accept connections from counters and proxy theme
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		fmt.Println("Socket Accepted:", local_port)

		// Establish WebSocket connection to VPS
		vpsURL := url.URL{Scheme: wss, Host: server, Path: address + target}
		query := vpsURL.Query()
		query.Set("port", remote_port)
		vpsURL.RawQuery = query.Encode()

		ws, _, err := websocket.DefaultDialer.Dial(vpsURL.String(), nil)
		if err != nil || ws == nil {
			fmt.Printf("Failed to connect to VPS: %v\n", err)
			conn.Close()
			continue
		}

		fmt.Println("WS connected:", remote_port)

		//because we first parse socket 20241 and 20242 so message has made since then
		//we send first  message for server, second for admin process
		if remote_port == "3309" {
			err = ws.WriteJSON(message)
			if err != nil {
				fmt.Printf("Failed to send DB message: %v\n", err)
				ws.Close()
				conn.Close()
				continue
			}
			fmt.Println("Sent First message DB")

			err = ws.WriteJSON(message)
			if err != nil {
				fmt.Printf("Failed to send DB message: %v\n", err)
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
			fmt.Printf("Error decode json: %v\n", err)
			return nil, nil
		}
	}

	return lengthBytes, messageBytes
}

func repeatMessage(ws *websocket.Conn, lengthBytes, messageBytes []byte) bool {
	err := ws.WriteMessage(websocket.BinaryMessage, lengthBytes)
	if err != nil {
		fmt.Printf("Failed to send LENGTH: %v\n", err)
		return false
	}

	err = ws.WriteMessage(websocket.TextMessage, messageBytes)
	if err != nil {
		fmt.Printf("Failed to send MESSAGE: %v\n", err)
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
					fmt.Printf("Failed to send data to WebSocket: %v\n", err)
					return
				}
			}
			if err != nil {
				fmt.Printf("TCP connection closed: %v\n", err)
				return
			}
		}
	}()

	// Forward data from WebSocket to TCP
	for {
		// Read from WebSocket
		_, message, err := ws.ReadMessage()
		if message != nil {
			// Write to TCP connection
			_, err = tcpConn.Write(message)
			if err != nil {
				fmt.Printf("Failed to write data to TCP: %v\n", err)
				return
			}
		}
		if err != nil {
			fmt.Printf("WebSocket connection closed: %v\n", err)
			return
		}
	}
}
