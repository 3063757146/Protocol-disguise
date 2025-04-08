package main

import (
	// "fmt"
	"io"
	"net"
)

func handleLocalConn(localConn net.Conn, serverAddr string) {
	defer localConn.Close()
	serverConn, _ := net.Dial("tcp", serverAddr)
	defer serverConn.Close()

	go io.Copy(serverConn, localConn)
	buf := make([]byte, 4096)

	for {
		n, err := localConn.Read(buf)
		if err != nil {
			return
		}
		data := Encode(AESEncrypt(buf[:n]))
		serverConn.Write(data)

		n2, err2 := serverConn.Read(buf)
		if err2 != nil {
			return
		}
		reply := AESDecrypt(Decode(buf[:n2]))
		localConn.Write(reply)
	}
}

// func main() {
// 	LoadConfig()
// 	ln, _ := net.Listen("tcp", "127.0.0.1:8080")
// 	fmt.Println("SMTP代理客户端启动")

// 	for {
// 		conn, _ := ln.Accept()
// 		go handleLocalConn(conn, "127.0.0.1:2525")
// 	}
// }
