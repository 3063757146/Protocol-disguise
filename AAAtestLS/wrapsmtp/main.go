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
	"net"
	"os"
	"strings"
	"time"
)

type Config struct {
	SMTPPort   string `json:"smtp_port"`
	ProxyPort  string `json:"proxy_port"`
	TargetAddr string `json:"target_addr"`
	AESKey     string `json:"aes_key"` // AES加密密钥
}

func main() {
	// 从配置文件加载配置
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 启动SMTP服务端
	go func() {
		listener, err := net.Listen("tcp", config.SMTPPort)
		if err != nil {
			fmt.Printf("Server start failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SMTP server listening on %s\n", config.SMTPPort)

		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("Accept error: %v\n", err)
				continue
			}
			go handleServerConn(conn, config)
		}
	}()

	// 启动本地代理客户端
	listener, err := net.Listen("tcp", config.ProxyPort)
	if err != nil {
		fmt.Printf("Proxy start failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Local proxy listening on %s\n", config.ProxyPort)

	// 启动测试客户端
	go testClient(config.ProxyPort, config.TargetAddr)

	// 启动一个定时器，5秒后自动关闭程序
	go func() {
		time.Sleep(5 * time.Second)
		fmt.Println("为了不影响后续演示，隧道将在5秒后自动关闭")
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Proxy accept error: %v\n", err)
			continue
		}
		go handleProxyConn(conn, config)
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

// 服务端处理逻辑
func handleServerConn(conn net.Conn, config Config) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	sendSMTPResponse(conn, "220 SMTP Gateway")
	readSMTPLine(reader) // HELO

	sendSMTPResponse(conn, "250 Hello")
	readSMTPLine(reader) // MAIL FROM

	sendSMTPResponse(conn, "250 OK")
	readSMTPLine(reader) // RCPT TO

	sendSMTPResponse(conn, "250 OK")
	readUntil(reader, "DATA\r\n")

	sendSMTPResponse(conn, "354 Send message")

	data := readSMTPData(reader)
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(data, "\r\n", ""))
	if err != nil {
		fmt.Printf("Base64 Decode error: %v\n", err)
		sendSMTPResponse(conn, "500 Invalid base64 format")
		return
	}

	// AES解密
	decrypted, err := aesDecrypt(decoded, []byte(config.AESKey))
	if err != nil {
		fmt.Printf("AES Decrypt error: %v\n", err)
		sendSMTPResponse(conn, "500 Decryption failed")
		return
	}

	// fmt.Printf("Decrypted data: %s\n", string(decrypted))

	// 解析目标信息
	payload := strings.SplitN(string(decrypted), "|", 3)
	if len(payload) != 3 {
		fmt.Printf("Invalid payload format: %#v\n", payload)
		sendSMTPResponse(conn, "500 Invalid payload format")
		return
	}
	// fmt.Printf("Parsed payload: host=%s, port=%s, data=%s\n", payload[0], payload[1], payload[2])

	// 正确拼接目标地址
	targetAddr := fmt.Sprintf("%s:%s", payload[0], payload[1])
	fmt.Printf("Connecting to: %s\n", targetAddr)

	// 设置连接超时
	targetConn, err := net.DialTimeout("tcp", targetAddr, 3*time.Second)
	if err != nil {
		fmt.Printf("Target connection failed: %v\n", err)
		sendSMTPResponse(conn, "500 Connection failed")
		return
	}
	defer targetConn.Close()

	// 转发数据
	if _, err := targetConn.Write([]byte(payload[2])); err != nil {
		fmt.Printf("Forward error: %v\n", err)
		sendSMTPResponse(conn, "500 Forward error")
		return
	}

	// 读取响应
	resp := make([]byte, 4096)
	targetConn.SetReadDeadline(time.Now().Add(5 * time.Second)) // 设置读取超时
	n, err := targetConn.Read(resp)
	if err != nil {
		if err.Error() == "read tcp "+targetAddr+": i/o timeout" {
			fmt.Printf("Read response timeout\n")
		} else {
			fmt.Printf("Read response error: %v\n", err)
		}
		sendSMTPResponse(conn, "500 Read error")
		return
	}

	// 封装响应
	encodedResp := base64.StdEncoding.EncodeToString(resp[:n])

	// AES加密
	encrypted, err := aesEncrypt([]byte(encodedResp), []byte(config.AESKey))
	if err != nil {
		fmt.Printf("AES Encrypt error: %v\n", err)
		sendSMTPResponse(conn, "500 Encryption failed")
		return
	}

	encodedEncrypted := base64.StdEncoding.EncodeToString(encrypted)
	sendSMTPResponse(conn, "250 OK\r\n"+encodedEncrypted+"\r\n.\r\n")
}

// 代理客户端处理逻辑
func handleProxyConn(localConn net.Conn, config Config) {
	defer localConn.Close()

	smtpConn, err := net.Dial("tcp", config.SMTPPort)
	if err != nil {
		fmt.Printf("SMTP connection failed: %v\n", err)
		return
	}
	defer smtpConn.Close()

	reader := bufio.NewReader(smtpConn)
	// 读取SMTP欢迎消息
	resp, err := readUntilTimeout(reader, "220 ", 3*time.Second)
	if err != nil {
		fmt.Printf("Read SMTP greeting error: %v\n", err)
		return
	}
	fmt.Printf("Received SMTP greeting: %s\n", resp)

	// 发送HELO命令
	fmt.Fprintf(smtpConn, "HELO proxy\r\n")
	resp, err = readUntilTimeout(reader, "250 Hello", 3*time.Second)
	if err != nil {
		fmt.Printf("HELO command error: %v\n", err)
		return
	}

	// 发送MAIL FROM命令
	fmt.Fprintf(smtpConn, "MAIL FROM:<proxy@local>\r\n")
	resp, err = readUntilTimeout(reader, "250 OK", 3*time.Second)
	if err != nil {
		fmt.Printf("MAIL FROM command error: %v\n", err)
		return
	}

	// 发送RCPT TO命令
	fmt.Fprintf(smtpConn, "RCPT TO:<remote@server>\r\n")
	resp, err = readUntilTimeout(reader, "250 OK", 3*time.Second)
	if err != nil {
		fmt.Printf("RCPT TO command error: %v\n", err)
		return
	}

	// 发送DATA命令
	fmt.Fprintf(smtpConn, "DATA\r\n")
	resp, err = readUntilTimeout(reader, "354 Send message", 3*time.Second)
	if err != nil {
		fmt.Printf("DATA command error: %v\n", err)
		return
	}

	// 读取本地数据
	buf := make([]byte, 1024)
	n, err := localConn.Read(buf)
	if err != nil && err != io.EOF {
		fmt.Printf("Local read error: %v\n", err)
		return
	}

	// 构造payload
	host, port, err := net.SplitHostPort(config.TargetAddr)
	if err != nil {
		fmt.Printf("Split target address error: %v\n", err)
		return
	}
	payload := fmt.Sprintf("%s|%s|%s", host, port, string(buf[:n]))
	fmt.Printf("Original payload: %s\n", payload)

	// AES加密
	encrypted, err := aesEncrypt([]byte(payload), []byte(config.AESKey))
	if err != nil {
		fmt.Printf("AES Encrypt error: %v\n", err)
		return
	}

	// Base64编码
	encoded := base64.StdEncoding.EncodeToString(encrypted)
	// fmt.Printf("Encoded payload: %s\n", encoded)
	// 发送数据
	fmt.Fprintf(smtpConn, "%s\r\n.\r\n", encoded)

	// 读取响应
	respData, err := readUntilTimeout(reader, "\r\n.\r\n", 5*time.Second)
	if err != nil {
		fmt.Printf("Read response error: %v\n", err)
		return
	}
	parts := strings.Split(respData, "\r\n")
	if len(parts) > 2 {
		// 检查并清理数据
		cleanedData := strings.TrimSpace(parts[2])
		if decoded, err := base64.StdEncoding.DecodeString(cleanedData); err == nil {
			// AES解密
			decrypted, err := aesDecrypt(decoded, []byte(config.AESKey))
			if err != nil {
				fmt.Printf("AES Decrypt error: %v\n", err)
				return
			}
			decodedData, err := base64.StdEncoding.DecodeString(string(decrypted))
			if err != nil {
				fmt.Println("Base64 decode failed:", err)
				return
			}

			// 将解密后的数据发送回测试客户端
			localConn.Write(decodedData)
		} else {
			fmt.Printf("Decode response error: %v\n", err)
			// 打印原始数据和清理后的数据以便调试
			fmt.Printf("Original data: %s\n", parts[2])
			fmt.Printf("Cleaned data: %s\n", cleanedData)
		}
	}
}

func aesEncrypt(plainText []byte, aesKey []byte) ([]byte, error) {
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

func aesDecrypt(cipherText []byte, aesKey []byte) ([]byte, error) {
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

// 测试客户端
func testClient(proxyPort, targetAddr string) {
	conn, err := net.Dial("tcp", proxyPort)
	if err != nil {
		fmt.Printf("Test client connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	// 构造HTTP请求
	httpRequest := "GET / HTTP/1.1\r\nHost: " + targetAddr + "\r\nConnection: close\r\n\r\n"
	fmt.Printf("Sending HTTP request: %s\n", httpRequest)

	// 发送HTTP请求
	_, err = conn.Write([]byte(httpRequest))
	if err != nil {
		fmt.Printf("Test client write failed: %v\n", err)
		return
	}

	// 使用通道来等待响应

	responseChan := make(chan []byte)
	go func() {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("Test client read failed: %v\n", err)
			responseChan <- nil
			return
		}
		responseChan <- buf[:n]
	}()

	// 等待响应或超时
	select {
	case response := <-responseChan:
		if response != nil {
			fmt.Printf("Test client received response: %s\n", string(response))
		}
	case <-time.After(8 * time.Second): // 增加超时时间
		fmt.Println("Test client timed out waiting for response")
	}
}

// 通用工具函数
func sendSMTPResponse(conn net.Conn, msg string) {
	conn.Write([]byte(msg + "\r\n"))
}

func readSMTPLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return line
}

func readSMTPData(r *bufio.Reader) string {
	var data strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "." {
			break
		}
		data.WriteString(line)
	}
	return data.String()
}

// 带超时的readUntil函数
func readUntilTimeout(r *bufio.Reader, delim string, timeout time.Duration) (string, error) {
	var result strings.Builder
	startTime := time.Now()
	for {
		b, err := r.ReadByte()
		if err != nil {
			return result.String(), err
		}
		result.WriteByte(b)
		if strings.HasSuffix(result.String(), delim) {
			return result.String(), nil
		}
		if time.Since(startTime) > timeout {
			return result.String(), fmt.Errorf("timeout while reading")
		}
	}
}

// 普通的readUntil函数
func readUntil(r *bufio.Reader, delim string) string {
	var result strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			break
		}
		result.WriteByte(b)
		if strings.HasSuffix(result.String(), delim) {
			break
		}
	}
	return result.String()
}
