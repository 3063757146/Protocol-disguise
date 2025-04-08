package main

// import (
// 	"bufio"
// 	"bytes"
// 	"encoding/base64"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net"
// 	"os"
// 	"strings"
// 	"time"
// )

// type Config struct {
// 	POP3Port   string `json:"pop3_port"`
// 	ProxyPort  string `json:"proxy_port"`
// 	TargetAddr string `json:"target_addr"`
// }

// func main() {
// 	// 从配置文件加载配置
// 	config, err := loadConfig("config.json")
// 	if err != nil {
// 		fmt.Printf("加载配置文件失败: %v\n", err)
// 		os.Exit(1)
// 	}

// 	// 启动POP3服务端
// 	go func() {
// 		listener, err := net.Listen("tcp", config.POP3Port)
// 		if err != nil {
// 			fmt.Printf("POP3 server start failed: %v\n", err)
// 			os.Exit(1)
// 		}
// 		fmt.Printf("POP3 server listening on %s\n", config.POP3Port)

// 		for {
// 			conn, err := listener.Accept()
// 			if err != nil {
// 				fmt.Printf("Accept error: %v\n", err)
// 				continue
// 			}
// 			go handlePOP3Connection(conn, config.TargetAddr)
// 		}
// 	}()

// 	// 启动本地代理
// 	listener, err := net.Listen("tcp", config.ProxyPort)
// 	if err != nil {
// 		fmt.Printf("Proxy start failed: %v\n", err)
// 		os.Exit(1)
// 	}
// 	fmt.Printf("Proxy listening on %s\n", config.ProxyPort)

// 	// 启动测试客户端
// 	go testPOP3Client(config.ProxyPort)

// 	// 启动一个定时器，5秒后自动关闭程序
// 	go func() {
// 		time.Sleep(5 * time.Second)
// 		fmt.Println("为了不影响后续演示 ，隧道将在5秒后自动关闭")
// 		os.Exit(0)
// 	}()

// 	for {
// 		conn, err := listener.Accept()
// 		if err != nil {
// 			fmt.Printf("Proxy accept error: %v\n", err)
// 			continue
// 		}
// 		go handleProxyConnection(conn, config.TargetAddr, config.POP3Port)
// 	}
// }

// func loadConfig(filename string) (Config, error) {
// 	data, err := os.ReadFile(filename)
// 	if err != nil {
// 		return Config{}, err
// 	}

// 	var config Config
// 	err = json.Unmarshal(data, &config)
// 	if err != nil {
// 		return Config{}, err
// 	}

// 	return config, nil
// }

// // POP3服务端处理逻辑
// func handlePOP3Connection(conn net.Conn, TargetAddr string) {
// 	defer conn.Close()
// 	reader := bufio.NewReader(conn)

// 	// POP3协议阶段
// 	sendPOP3Response(conn, "+OK POP3 proxy ready")

// 	// 认证阶段
// 	for {
// 		line, err := reader.ReadString('\n')
// 		if err != nil {
// 			fmt.Printf("Read error: %v\n", err)
// 			return
// 		}
// 		line = strings.TrimSpace(line)

// 		if strings.HasPrefix(line, "USER ") {
// 			sendPOP3Response(conn, "+OK")
// 		} else if strings.HasPrefix(line, "PASS ") {
// 			sendPOP3Response(conn, "+OK Logged in")
// 			break
// 		} else {
// 			sendPOP3Response(conn, "-ERR Expected authentication")
// 		}
// 	}

// 	// 事务处理阶段
// 	sendPOP3Response(conn, "+OK Entering transaction")

// 	for {
// 		line, err := reader.ReadString('\n')
// 		if err != nil {
// 			fmt.Printf("Read error: %v\n", err)
// 			return
// 		}
// 		line = strings.TrimSpace(line)

// 		switch {
// 		case line == "STAT":
// 			sendPOP3Response(conn, "+OK 1 1024") // 伪造邮件统计

// 		case strings.HasPrefix(line, "RETR "):
// 			// 从代理获取实际数据
// 			proxyData := fetchFromTarget(TargetAddr)
// 			fmt.Println("--", proxyData)

// 			encoded := base64.StdEncoding.EncodeToString(proxyData)
// 			if _, err := conn.Write([]byte(fmt.Sprintf("+OK %d bytes", len(encoded)))); err != nil {
// 				fmt.Printf("Write error: %v\n", err)
// 				return
// 			}
// 			if _, err := conn.Write([]byte(" " + encoded + "\r\n.\r\n")); err != nil {
// 				fmt.Printf("Write error: %v\n", err)
// 				return
// 			}

// 		case line == "QUIT":
// 			sendPOP3Response(conn, "+OK Bye")
// 			return

// 		default:
// 			sendPOP3Response(conn, "-ERR Unsupported command")
// 		}
// 	}
// }

// // 实际获取目标数据的函数
// func fetchFromTarget(target string) []byte {
// 	conn, err := net.DialTimeout("tcp", target, 3*time.Second)
// 	if err != nil {
// 		return []byte("HTTP/1.1 500 Connection failed\r\n\r\n")
// 	}
// 	defer conn.Close()

// 	request := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
// 	if _, err := conn.Write([]byte(request)); err != nil {
// 		return []byte("HTTP/1.1 500 Send request failed\r\n\r\n")
// 	}

// 	buf := new(bytes.Buffer)
// 	if _, err := io.Copy(buf, conn); err != nil {
// 		return []byte("HTTP/1.1 500 Read response failed\r\n\r\n")
// 	}
// 	return buf.Bytes()
// }

// // 本地代理处理
// func handleProxyConnection(localConn net.Conn, targetAddr string, POP3Port string) {
// 	defer localConn.Close()

// 	pop3Conn, err := net.Dial("tcp", POP3Port)
// 	if err != nil {
// 		fmt.Printf("POP3 connection failed: %v\n", err)
// 		return
// 	}
// 	defer pop3Conn.Close()
// 	reader := bufio.NewReader(pop3Conn)

// 	if resp := readPOP3Response(reader); !strings.HasPrefix(resp, "+OK") {
// 		fmt.Printf("Invalid welcome response: %s\n", resp)
// 		return
// 	}
// 	if _, err := pop3Conn.Write([]byte("USER dummy\r\n")); err != nil {
// 		fmt.Printf("Write error: %v\n", err)
// 		return
// 	}
// 	if resp := readPOP3Response(reader); !strings.HasPrefix(resp, "+OK") {
// 		fmt.Printf("Invalid USER response: %s\n", resp)
// 		return
// 	}
// 	if _, err := pop3Conn.Write([]byte("PASS secret\r\n")); err != nil {
// 		fmt.Printf("Write error: %v\n", err)
// 		return
// 	}
// 	if resp := readPOP3Response(reader); !strings.HasPrefix(resp, "+OK") {
// 		fmt.Printf("Invalid PASS response: %s\n", resp)
// 		return
// 	}

// 	if _, err := pop3Conn.Write([]byte("RETR 1\r\n")); err != nil {
// 		fmt.Printf("Write error: %v\n", err)
// 		return
// 	}

// 	header, err := reader.ReadString('\n')
// 	if err != nil {
// 		fmt.Printf("Read error: %v\n", err)
// 		return
// 	}
// 	if !strings.HasPrefix(header, "+OK") {
// 		fmt.Printf("Invalid response: %s\n", header)
// 		return
// 	}

// 	var encoded strings.Builder
// 	for {
// 		line, err := reader.ReadString('\n')
// 		if err != nil {
// 			fmt.Printf("Read error: %v\n", err)
// 			return
// 		}
// 		line = strings.TrimSpace(line)
// 		if line == "." {
// 			break
// 		}
// 		encoded.WriteString(line)
// 	}
// 	encodedContent := encoded.String()

// 	if strings.HasPrefix(encodedContent, "+OK ") {
// 		parts := strings.SplitN(encodedContent, " ", 4)
// 		if len(parts) >= 4 {
// 			encodedContent = parts[3]
// 		}
// 	}

// 	decoded, err := base64.StdEncoding.DecodeString(encodedContent)
// 	if err != nil {
// 		fmt.Printf("Decode error: %v\n", err)
// 		return
// 	}

// 	if _, err := localConn.Write(decoded); err != nil {
// 		fmt.Printf("Write error: %v\n", err)
// 		return
// 	}
// }

// // 测试客户端
// func testPOP3Client(proxyPort string) {
// 	conn, err := net.Dial("tcp", proxyPort)
// 	if err != nil {
// 		fmt.Printf("Test client connection failed: %v\n", err)
// 		return
// 	}
// 	defer conn.Close()

// 	buf := make([]byte, 4096)
// 	n, err := conn.Read(buf)
// 	if err != nil {
// 		fmt.Printf("Read error: %v\n", err)
// 		return
// 	}
// 	fmt.Printf("----------(Test Client) Received:\n%s\n", buf[:n])
// }

// // 工具函数
// func sendPOP3Response(conn net.Conn, msg string) {
// 	if _, err := conn.Write([]byte(msg + "\r\n")); err != nil {
// 		fmt.Printf("Write error: %v\n", err)
// 	}
// }

// func readPOP3Response(reader *bufio.Reader) string {
// 	resp, err := reader.ReadString('\n')
// 	if err != nil {
// 		fmt.Printf("Read error: %v\n", err)
// 		return ""
// 	}
// 	return strings.TrimSpace(resp)
// }
