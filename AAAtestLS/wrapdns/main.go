package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/miekg/dns"
)

type Config struct {
	DNSServer    string `json:"dns_server"`    // 服务端监听的DNS端口
	ProxyPort    string `json:"proxy_port"`    // 本地代理监听的端口
	TargetServer string `json:"target_server"` // 目标服务器地址
	AESKey       string `json:"aes_key"`       // AES加密密钥
}

var aesKey []byte

func main() {
	fmt.Println("开始------------")

	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	aesKey = []byte(config.AESKey)
	if len(aesKey) != 16 {
		log.Fatalf("AES 密钥长度必须是16字节")
	}

	go startDNSServer(config.DNSServer, config.TargetServer)

	time.Sleep(time.Second)

	go startProxyServer(config.ProxyPort, config.DNSServer)

	time.Sleep(time.Second)

	startTestClient(config.ProxyPort)

	time.Sleep(time.Second * 2)
}

func loadConfig(filename string) (Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, err
	}
	var config Config
	err = json.Unmarshal(data, &config)
	return config, err
}

// 本地代理客户端处理逻辑
func startProxyServer(proxyPort, dnsServerAddr string) {
	listener, err := net.Listen("tcp", proxyPort)
	if err != nil {
		log.Fatalf("本地代理启动失败: %v", err)
	}
	defer listener.Close()
	log.Printf("本地代理监听在 %s", proxyPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接受连接失败: %v", err)
			continue
		}
		go handleProxyConn(conn, dnsServerAddr)
	}
}

func handleProxyConn(localConn net.Conn, dnsServerAddr string) {
	defer localConn.Close()

	// 读取HTTP请求
	buf := make([]byte, 1024)
	n, err := localConn.Read(buf)
	if err != nil {
		log.Printf("读取HTTP请求失败: %v", err)
		return
	}

	// 构造HTTP请求数据
	httpRequest := string(buf[:n])
	fmt.Printf("接收到HTTP请求: %s\n", httpRequest)

	// 使用AES加密和Base64编码
	encrypted, err := aesEncrypt([]byte(httpRequest))
	if err != nil {
		log.Printf("AES加密失败: %v", err)
		return
	}

	// 封装成DNS请求
	dnsMsg, err := encapsulateDataToDNSQuery(encrypted)
	if err != nil {
		log.Printf("封装DNS请求失败: %v", err)
		return
	}

	// 发送DNS请求
	conn, err := net.Dial("udp", dnsServerAddr)
	if err != nil {
		log.Printf("连接DNS服务器失败: %v", err)
		return
	}
	defer conn.Close()

	query, err := dnsMsg.Pack()
	if err != nil {
		log.Printf("打包DNS请求失败: %v", err)
		return
	}

	_, err = conn.Write(query)
	if err != nil {
		log.Printf("发送DNS请求失败: %v", err)
		return
	}

	// 接收DNS响应
	buf = make([]byte, 1024)
	n, err = conn.Read(buf)
	if err != nil {
		log.Printf("接收DNS响应失败: %v", err)
		return
	}

	response := &dns.Msg{}
	err = response.Unpack(buf[:n])
	if err != nil {
		log.Printf("解包DNS响应失败: %v", err)
		return
	}

	// 提取响应数据
	responseData, err := extractDataFromDNSQuery(response)
	if err != nil {
		log.Printf("解析DNS响应失败: %v", err)
		return
	}

	// 解密响应数据
	decrypted, err := aesDecrypt(responseData)
	if err != nil {
		log.Printf("AES解密失败: %v", err)
		return
	}

	// 返回响应给测试客户端
	localConn.Write(decrypted)
}

// 服务端处理逻辑
func startDNSServer(dnsServerAddr, targetServer string) {
	addr, err := net.ResolveUDPAddr("udp", dnsServerAddr)
	if err != nil {
		log.Fatalf("解析DNS服务器地址失败: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("启动DNS服务器失败: %v", err)
	}
	defer conn.Close()

	log.Printf("DNS服务器启动在 %s", dnsServerAddr)

	for {
		buf := make([]byte, 1024)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("接收DNS请求失败: %v", err)
			continue
		}

		request := &dns.Msg{}
		if err := request.Unpack(buf[:n]); err != nil {
			log.Printf("解包DNS请求失败: %v", err)
			continue
		}

		response, err := handleDNSRequest(request, targetServer)
		if err != nil {
			log.Printf("处理DNS请求失败: %v", err)
			continue
		}

		resp, err := response.Pack()
		if err != nil {
			log.Printf("打包DNS响应失败: %v", err)
			continue
		}

		if _, err := conn.WriteToUDP(resp, addr); err != nil {
			log.Printf("发送DNS响应失败: %v", err)
		}
	}
}

func handleDNSRequest(request *dns.Msg, targetServer string) (*dns.Msg, error) {
	data, err := extractDataFromDNSQuery(request)
	if err != nil {
		log.Printf("提取DNS数据失败: %v", err)
		return nil, err
	}

	if data != nil {
		decrypted, err := aesDecrypt(data)
		if err != nil {
			return nil, err
		}

		responseData, err := forwardDataToTargetServer(decrypted, targetServer)
		if err != nil {
			return nil, err
		}

		encrypted, err := aesEncrypt(responseData)
		if err != nil {
			return nil, err
		}

		responseMsg, err := encapsulateDataToDNSQuery(encrypted)
		if err != nil {
			return nil, err
		}
		responseMsg.SetReply(request)
		return responseMsg, nil
	}

	response := &dns.Msg{}
	response.SetReply(request)
	return response, nil
}

func forwardDataToTargetServer(data []byte, targetServer string) ([]byte, error) {
	addr, err := net.ResolveUDPAddr("udp", targetServer)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// 测试客户端
func startTestClient(proxyPort string) {
	httpRequest := "GET / HTTP/1.1\r\nHost: example.com\r\nUser-Agent: Go-http-client/1.1\r\n\r\n"
	fmt.Printf("正在测试发送数据: %s\n", httpRequest)

	conn, err := net.Dial("tcp", proxyPort)
	if err != nil {
		log.Fatalf("连接本地代理失败: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(httpRequest))
	if err != nil {
		log.Fatalf("发送HTTP请求失败: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("接收响应失败: %v", err)
	}

	fmt.Printf("收到目标服务器返回数据: %s\n", string(buf[:n]))
}

// 加密和解密函数
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

// DNS封装和解封装函数
func encapsulateDataToDNSQuery(data []byte) (*dns.Msg, error) {
	encodedData := base64.StdEncoding.EncodeToString(data)

	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = []dns.Question{
		{
			Name:   "encapsulated.example.com.",
			Qtype:  dns.TypeTXT,
			Qclass: dns.ClassINET,
		},
	}

	txtRecord := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   "encapsulated.example.com.",
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		Txt: []string{encodedData},
	}

	msg.Answer = append(msg.Answer, txtRecord)
	return msg, nil
}

func extractDataFromDNSQuery(msg *dns.Msg) ([]byte, error) {
	for _, answer := range msg.Answer {
		if txt, ok := answer.(*dns.TXT); ok {
			if len(txt.Txt) > 0 {
				raw, err := base64.StdEncoding.DecodeString(txt.Txt[0])
				if err != nil {
					return nil, err
				}
				return raw, nil
			}
		}
	}
	return nil, fmt.Errorf("DNS数据中未找到封装内容")
}
