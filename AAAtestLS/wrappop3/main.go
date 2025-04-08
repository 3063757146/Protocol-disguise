package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type Config struct {
	POP3Port   string `json:"pop3_port"`
	ProxyPort  string `json:"proxy_port"`
	TargetAddr string `json:"target_addr"`
	AESKey     string `json:"aes_key"`
}

var aesKey []byte

func main() {
	// 从配置文件加载配置
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}
	aesKey = []byte(config.AESKey)
	if len(aesKey) != 16 {
		log.Fatalf("AES 密钥长度必须是16字节")
	}

	// 启动POP3服务端
	go func() {
		listener, err := net.Listen("tcp", config.POP3Port)
		if err != nil {
			fmt.Printf("POP3 server start failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("POP3 server listening on %s\n", config.POP3Port)

		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("Accept error: %v\n", err)
				continue
			}
			go handlePOP3Connection(conn, config.TargetAddr)
		}
	}()

	// 启动本地代理
	listener, err := net.Listen("tcp", config.ProxyPort)
	if err != nil {
		fmt.Printf("Proxy start failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Proxy listening on %s\n", config.ProxyPort)

	// 启动测试客户端
	go testPOP3Client(config.ProxyPort)

	// 启动一个定时器，5秒后自动关闭程序
	go func() {
		time.Sleep(5 * time.Second)
		fmt.Println("为了不影响后续演示 ，隧道将在5秒后自动关闭")
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Proxy accept error: %v\n", err)
			continue
		}
		go handleProxyConnection(conn, config.TargetAddr, config.POP3Port)
	}
}

func loadConfig(filename string) (Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

// POP3服务端处理逻辑
func handlePOP3Connection(conn net.Conn, targetAddr string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	// POP3 协议问候
	sendPOP3Response(conn, "+OK POP3 proxy ready")

	// 认证阶段
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			return
		}
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "USER ") {
			sendPOP3Response(conn, "+OK")
		} else if strings.HasPrefix(line, "PASS ") {
			sendPOP3Response(conn, "+OK Logged in")
			break
		} else {
			sendPOP3Response(conn, "-ERR Expected authentication")
		}
	}

	// 事务处理阶段
	sendPOP3Response(conn, "+OK Entering transaction")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			return
		}
		line = strings.TrimSpace(line)

		switch {
		case line == "STAT":
			sendPOP3Response(conn, "+OK 1 1024") // 伪造邮件统计

		case strings.HasPrefix(line, "RETR "):
			// 从代理获取实际数据
			proxyData := fetchFromTarget(targetAddr)
			// 先对原始数据进行 AES 加密
			encrypted, err := aesEncrypt(proxyData)
			if err != nil {
				fmt.Printf("Encryption error: %v\n", err)
				return
			}
			// 再进行 Base64 编码
			encoded := base64.StdEncoding.EncodeToString(encrypted)
			// fmt.Println(encoded)
			if _, err := conn.Write([]byte(fmt.Sprintf("+OK %d bytes", len(encoded)))); err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
			if _, err := conn.Write([]byte(" " + encoded + "\r\n.\r\n")); err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}

		case line == "QUIT":
			sendPOP3Response(conn, "+OK Bye")
			return

		default:
			sendPOP3Response(conn, "-ERR Unsupported command")
		}
	}
}

// 实际获取目标数据的函数
func fetchFromTarget(target string) []byte {
	conn, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		return []byte("HTTP/1.1 500 Connection failed\r\n\r\n")
	}
	defer conn.Close()

	request := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
	if _, err := conn.Write([]byte(request)); err != nil {
		return []byte("HTTP/1.1 500 Send request failed\r\n\r\n")
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, conn); err != nil {
		return []byte("HTTP/1.1 500 Read response failed\r\n\r\n")
	}
	return buf.Bytes()
}

// 本地代理处理，从 POP3 服务端获取数据后，进行 Base64 解码，再 AES 解密后转发给客户端
func handleProxyConnection(localConn net.Conn, targetAddr string, POP3Port string) {
	defer localConn.Close()

	pop3Conn, err := net.Dial("tcp", POP3Port)
	if err != nil {
		fmt.Printf("POP3 connection failed: %v\n", err)
		return
	}
	defer pop3Conn.Close()
	reader := bufio.NewReader(pop3Conn)

	if resp := readPOP3Response(reader); !strings.HasPrefix(resp, "+OK") {
		fmt.Printf("Invalid welcome response: %s\n", resp)
		return
	}
	if _, err := pop3Conn.Write([]byte("USER dummy\r\n")); err != nil {
		fmt.Printf("Write error: %v\n", err)
		return
	}
	if resp := readPOP3Response(reader); !strings.HasPrefix(resp, "+OK") {
		fmt.Printf("Invalid USER response: %s\n", resp)
		return
	}
	if _, err := pop3Conn.Write([]byte("PASS secret\r\n")); err != nil {
		fmt.Printf("Write error: %v\n", err)
		return
	}
	if resp := readPOP3Response(reader); !strings.HasPrefix(resp, "+OK") {
		fmt.Printf("Invalid PASS response: %s\n", resp)
		return
	}

	if _, err := pop3Conn.Write([]byte("RETR 1\r\n")); err != nil {
		fmt.Printf("Write error: %v\n", err)
		return
	}

	header, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return
	}
	if !strings.HasPrefix(header, "+OK") {
		fmt.Printf("Invalid response: %s\n", header)
		return
	}

	var encoded strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "." {
			break
		}
		encoded.WriteString(line)
	}
	encodedContent := encoded.String()

	if strings.HasPrefix(encodedContent, "+OK ") {
		parts := strings.SplitN(encodedContent, " ", 4)
		if len(parts) >= 4 {
			encodedContent = parts[3]
		}
	}

	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(encodedContent)
	if err != nil {
		fmt.Printf("Decode error: %v\n", err)
		return
	}

	// AES 解密
	decrypted, err := aesDecrypt(decoded)
	if err != nil {
		fmt.Printf("Decrypt error: %v\n", err)
		return
	}

	if _, err := localConn.Write(decrypted); err != nil {
		fmt.Printf("Write error: %v\n", err)
		return
	}
}

// 测试客户端，用于展示从代理获取的数据
func testPOP3Client(proxyPort string) {
	conn, err := net.Dial("tcp", proxyPort)
	if err != nil {
		fmt.Printf("Test client connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return
	}
	fmt.Printf("----------(Test Client) Received:\n%s\n", buf[:n])
}

// 工具函数：发送 POP3 响应
func sendPOP3Response(conn net.Conn, msg string) {
	if _, err := conn.Write([]byte(msg + "\r\n")); err != nil {
		fmt.Printf("Write error: %v\n", err)
	}
}

// 工具函数：读取 POP3 响应
func readPOP3Response(reader *bufio.Reader) string {
	resp, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return ""
	}
	return strings.TrimSpace(resp)
}

// AES 加密函数：CBC 模式，使用固定 IV（取自密钥前16字节）
func aesEncrypt(plainText []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	padding := aes.BlockSize - len(plainText)%aes.BlockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	plainText = append(plainText, padtext...)

	iv := aesKey[:aes.BlockSize]
	blockMode := cipher.NewCBCEncrypter(block, iv)
	cipherText := make([]byte, len(plainText))
	blockMode.CryptBlocks(cipherText, plainText)
	return cipherText, nil
}

// AES 解密函数：CBC 模式，使用固定 IV
func aesDecrypt(cipherText []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	iv := aesKey[:aes.BlockSize]
	blockMode := cipher.NewCBCDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	blockMode.CryptBlocks(plainText, cipherText)

	length := len(plainText)
	unpadding := int(plainText[length-1])
	return plainText[:(length - unpadding)], nil
}
