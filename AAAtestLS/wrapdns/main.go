package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"time"

	"encoding/json"
	"os"

	"github.com/miekg/dns"
)

type Config struct {
	DNSServer    string `json:"dns_server"`
	TargetServer string `json:"target_server"`
}

func main() {
	// 从配置文件加载配置
	fmt.Println("开始------------")
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 使用 goroutine 启动 DNS 服务器
	go startDNSServer(config.DNSServer, config.TargetServer)

	// 等待服务器启动
	time.Sleep(time.Second)

	// 启动 DNS 客户端
	startDNSClient(config.DNSServer)

	// 等待客户端完成
	time.Sleep(time.Second * 2)
}

// 从配置文件加载配置
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

// 启动 DNS 服务器
func startDNSServer(dnsServerAddr, targetServer string) {
	// 创建 UDP 连接
	addr, err := net.ResolveUDPAddr("udp", dnsServerAddr)
	if err != nil {
		log.Fatalf("Error resolving DNS server address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Error starting DNS server: %v", err)
	}
	defer conn.Close()

	log.Printf("DNS server started on %s", dnsServerAddr)

	for {
		// 创建缓冲区
		buf := make([]byte, 1024)
		// 接收 DNS 请求
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error receiving DNS request: %v", err)
			continue
		}

		// 创建新的 DNS 消息
		request := &dns.Msg{}
		if err := request.Unpack(buf[:n]); err != nil {
			log.Printf("Error unpacking DNS request: %v", err)
			continue
		}

		// 处理 DNS 请求
		response, err := handleDNSRequest(request, targetServer)
		if err != nil {
			log.Printf("Error handling DNS request: %v", err)
			continue
		}

		// 打包 DNS 响应
		resp, err := response.Pack()
		if err != nil {
			log.Printf("Error packing DNS response: %v", err)
			continue
		}

		// 发送 DNS 响应
		if _, err := conn.WriteToUDP(resp, addr); err != nil {
			log.Printf("Error sending DNS response: %v", err)
		}
	}
}

// / 处理 DNS 请求
func handleDNSRequest(request *dns.Msg, targetServer string) (*dns.Msg, error) {
	// 判断是否为封装的数据
	data, err := extractDataFromDNSQuery(request)
	if err != nil {
		log.Printf("Error extracting data from DNS query: %v", err)
		return nil, err
	}

	if data != nil {
		// log.Printf("Received encapsulated data: %s", string(data))

		// 将数据发送到 target_server 并接收响应
		responseData, err := forwardDataToTargetServer(data, targetServer)
		if err != nil {
			log.Printf("Error forwarding data to target server: %v", err)
			return nil, err
		}

		// 构造响应消息
		responseMsg, err := encapsulateDataToDNSQuery(responseData)
		if err != nil {
			log.Printf("Error encapsulating response data: %v", err)
			return nil, err
		}
		responseMsg.SetReply(request)
		return responseMsg, nil
	}

	// 若不是封装的数据，按正常 DNS 流程处理
	response := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 request.Id,
			Response:           true,
			Opcode:             request.Opcode,
			Authoritative:      false,
			Truncated:          false,
			RecursionDesired:   request.RecursionDesired,
			RecursionAvailable: true,
			AuthenticatedData:  false,
			CheckingDisabled:   false,
			Rcode:              dns.RcodeSuccess,
		},
		Question: request.Question,
	}

	return response, nil
}

// 将数据转发到 target_server 并接收响应
func forwardDataToTargetServer(data []byte, targetServer string) ([]byte, error) {
	// 解析 target_server 地址
	addr, err := net.ResolveUDPAddr("udp", targetServer)
	if err != nil {
		return nil, err
	}

	// 创建 UDP 连接
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// 发送数据
	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(5 * time.Second)) // 5秒超时

	// 接收响应数据
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
			return nil, fmt.Errorf("timeout while waiting for response from target server")
		}
		return nil, err
	}

	return buf[:n], nil
}

// 启动 DNS 客户端
func startDNSClient(dnsServerAddr string) {
	// 模拟其他协议的数据
	httpRequest := "GET / HTTP/1.1\r\nHost: example.com\r\nUser-Agent: Go-http-client/1.1\r\n\r\n"
	fmt.Printf("-------------正在测试通过dns隧道向目标服务器发送：: %s\n", httpRequest)
	otherProtocolData := []byte(httpRequest)

	// 封装数据到 DNS 查询
	dnsMsg, err := encapsulateDataToDNSQuery(otherProtocolData)
	if err != nil {
		log.Fatalf("Error encapsulating data: %v", err)
	}

	// 创建 UDP 连接
	conn, err := net.Dial("udp", dnsServerAddr) // 连接到指定的 DNS 服务器
	if err != nil {
		log.Fatalf("Error connecting to DNS server: %v", err)
	}
	defer conn.Close()

	// 打包 DNS 查询消息
	query, err := dnsMsg.Pack()
	if err != nil {
		log.Fatalf("Error packing DNS query: %v", err)
	}

	// 发送 DNS 查询
	_, err = conn.Write(query)
	if err != nil {
		log.Fatalf("Error sending DNS query: %v", err)
	}

	// 接收 DNS 响应
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("Error receiving DNS response: %v", err)
	}

	// 解包 DNS 响应
	response := &dns.Msg{}
	err = response.Unpack(buf[:n])
	if err != nil {
		log.Fatalf("Error unpacking DNS response: %v", err)
	}

	// 解析响应中的封装数据
	responseData, err := extractDataFromDNSQuery(response)
	if err != nil {
		log.Fatalf("Error extracting data from DNS response: %v", err)
	}

	fmt.Printf("收到返回数据Received response data: %s\n", string(responseData))
}

// 封装其他协议数据到 DNS 查询
func encapsulateDataToDNSQuery(data []byte) (*dns.Msg, error) {
	encodedData := base64.StdEncoding.EncodeToString(data)
	// fmt.Printf("Encoded data: %s\n", data)
	// fmt.Printf("Encapsulating data: %s\n", encodedData)

	// 使用TXT记录来封装数据
	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, 1)
	msg.Question[0] = dns.Question{
		Name:   "encapsulated.example.com.",
		Qtype:  dns.TypeTXT,
		Qclass: dns.ClassINET,
	}

	// 添加TXT记录
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

// 从 DNS 查询中解析封装的数据
func extractDataFromDNSQuery(msg *dns.Msg) ([]byte, error) {
	for _, answer := range msg.Answer {
		if txt, ok := answer.(*dns.TXT); ok {
			if len(txt.Txt) > 0 {
				return base64.StdEncoding.DecodeString(txt.Txt[0])
			}
		}
	}

	return nil, fmt.Errorf("no encapsulated data found in DNS message")
}
