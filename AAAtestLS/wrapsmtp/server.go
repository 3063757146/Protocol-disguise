package main

import (
	"fmt"
	"net"
)

func handleConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		data := Decode(AESDecrypt(buf[:n]))
		fmt.Printf("收到数据: %s\n", string(data))

		reply := "Server收到: " + string(data)
		sendData := Encode(AESEncrypt([]byte(reply)))
		conn.Write(sendData)
	}
}

// func main() {
// 	LoadConfig()
// 	ln, _ := net.Listen("tcp", ":2525")
// 	fmt.Println("SMTP代理服务端启动")

// 	for {
// 		conn, _ := ln.Accept()
// 		go handleConn(conn)
// 	}
// }
