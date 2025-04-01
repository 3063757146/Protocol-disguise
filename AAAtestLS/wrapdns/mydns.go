package main

// import (
// 	"bytes"
// 	"context"
// 	"encoding/base64"
// 	"errors"
// 	"gost"
// 	"net"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/go-log/log"
// 	"github.com/miekg/dns"
// )

// var ( //声明一个全局的 DNS 解析器变量
// 	defaultResolver gost.Resolver
// )

// type dnsHandler struct {
// 	options *gost.HandlerOptions //用于存储处理程序的一些配置选项
// }

// // 修改 dnsHandler 的 Handle 方法以处理封装的数据
// // 该方法用于处理接收到的 DNS 连接，判断是否为封装的数据并进行相应处理
// func (h *dnsHandler) myHandle(conn net.Conn) {
// 	// 确保在函数结束时关闭连接，避免资源泄漏
// 	defer conn.Close()

// 	// 从内存池获取一个字节切片，用于存储从连接中读取的数据
// 	b := mPool.Get().([]byte)
// 	// 确保在函数结束时将字节切片放回内存池，以便复用
// 	defer mPool.Put(b)

// 	// 从连接中读取数据到字节切片
// 	n, err := conn.Read(b)
// 	if err != nil {
// 		// 如果读取数据时发生错误，记录错误日志
// 		log.Logf("[dns] %s - %s: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 		return
// 	}

// 	// 创建一个新的 DNS 消息对象
// 	mq := &dns.Msg{}
// 	// 解包接收到的数据到 DNS 消息对象中
// 	if err = mq.Unpack(b[:n]); err != nil {
// 		// 如果解包数据时发生错误，记录错误日志并返回
// 		log.Logf("[dns] %s - %s request unpack: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 		return
// 	}

// 	// 记录 DNS 请求消息的头部信息
// 	log.Logf("[dns] %s -> %s: %s", conn.RemoteAddr(), conn.LocalAddr(), h.dumpMsgHeader(mq))
// 	// 如果开启调试模式，记录完整的 DNS 请求消息
// 	if Debug {
// 		log.Logf("[dns] %s >>> %s: %s", conn.RemoteAddr(), conn.LocalAddr(), mq.String())
// 	}

// 	// 尝试从 DNS 查询消息中提取封装的数据
// 	data, err := extractDataFromDNSQuery(mq)
// 	if err != nil {
// 		// 如果提取数据时发生错误，记录错误日志
// 		log.Logf("[dns] %s - %s extract data: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 	} else if data != nil {
// 		// 如果成功提取到封装的数据，处理该数据
// 		log.Logf("[dns] %s - %s received encapsulated data: %s", conn.RemoteAddr(), conn.LocalAddr(), string(data))

// 		// 构造响应数据，这里简单地在原始数据前加上 "Data received: "
// 		responseData := []byte("Data received: " + string(data))
// 		// 将响应数据封装到 DNS 查询消息中
// 		responseMsg, err := encapsulateDataToDNSQuery(responseData)
// 		if err != nil {
// 			// 如果封装响应数据时发生错误，记录错误日志并返回
// 			log.Logf("[dns] %s - %s encapsulate response data: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 			return
// 		}
// 		// 将响应消息设置为对原始查询消息的回复
// 		responseMsg.SetReply(mq)

// 		// 将响应消息打包成字节切片，以便发送
// 		reply, err := responseMsg.Pack()
// 		if err != nil {
// 			// 如果打包响应消息时发生错误，记录错误日志并返回
// 			log.Logf("[dns] %s - %s pack response: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 			return
// 		}

// 		// 将打包好的响应消息写回到连接中
// 		if _, err = conn.Write(reply); err != nil {
// 			// 如果写入响应消息时发生错误，记录错误日志
// 			log.Logf("[dns] %s - %s write response: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 		}
// 		return
// 	}

// 	// 若不是封装的数据，按正常 DNS 流程处理
// 	// 记录处理开始时间，用于计算 DNS 交换的往返时间
// 	start := time.Now()

// 	// 获取 DNS 解析器，如果未配置则使用默认解析器
// 	resolver := h.options.Resolver
// 	if resolver == nil {
// 		resolver = defaultResolver
// 	}
// 	// 使用解析器交换 DNS 消息并获取回复
// 	reply, err := resolver.Exchange(context.Background(), b[:n])
// 	if err != nil {
// 		// 如果 DNS 交换过程中发生错误，记录错误日志并返回
// 		log.Logf("[dns] %s - %s exchange: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 		return
// 	}
// 	// 计算 DNS 交换的往返时间
// 	rtt := time.Since(start)

// 	// 创建一个新的 DNS 消息对象用于存储回复
// 	mr := &dns.Msg{}
// 	// 解包回复数据到 DNS 消息对象中
// 	if err = mr.Unpack(reply); err != nil {
// 		// 如果解包回复消息时发生错误，记录错误日志并返回
// 		log.Logf("[dns] %s - %s reply unpack: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 		return
// 	}
// 	// 记录 DNS 回复消息的头部信息和往返时间
// 	log.Logf("[dns] %s <- %s: %s [%s]",
// 		conn.RemoteAddr(), conn.LocalAddr(), h.dumpMsgHeader(mr), rtt)
// 	// 如果开启调试模式，记录完整的 DNS 回复消息
// 	if Debug {
// 		log.Logf("[dns] %s <<< %s: %s", conn.RemoteAddr(), conn.LocalAddr(), mr.String())
// 	}

// 	// 将 DNS 回复消息写回连接
// 	if _, err = conn.Write(reply); err != nil {
// 		// 如果写入回复消息时发生错误，记录错误日志
// 		log.Logf("[dns] %s - %s reply unpack: %v", conn.RemoteAddr(), conn.LocalAddr(), err)
// 	}
// }

// // 封装其他协议数据到 DNS 查询
// // 该函数用于将其他协议的数据封装到 DNS 查询消息中，以实现将其他协议流量伪装成 DNS 流量
// func encapsulateDataToDNSQuery(data []byte) (*dns.Msg, error) {
// 	// 对传入的其他协议数据进行 Base64 编码
// 	// Base64 编码可以将二进制数据转换为可打印的 ASCII 字符，方便嵌入到 DNS 域名中
// 	encodedData := base64.StdEncoding.EncodeToString(data)
// 	// 构造一个带有特定前缀的域名，用于标记这是封装的数据
// 	// 这里使用 "encapsulated." 作为前缀，方便后续在接收端识别
// 	domain := "encapsulated." + encodedData + ".example.com."
// 	// 	encodedData部分是原始数据经过Base64编码后的字符串，这样做的目的是为了确保即使是二进制数据也可以安全地嵌入到域名中，因为域名只能包含特定范围内的字符。
// 	//  encapsulated.作为前缀，是一个标识符，用于帮助接收方识别出这是一个被封装的数据包，而不是普通的DNS查询。
// 	// .example.com.作为后缀，是为了使整个字符串符合域名的标准格式要求。

// 	// 创建一个新的 DNS 消息对象
// 	m := new(dns.Msg)
// 	// 为 DNS 消息分配一个唯一的 ID
// 	m.Id = dns.Id()
// 	// 设置 DNS 查询期望递归解析
// 	m.RecursionDesired = true
// 	// 初始化 DNS 查询的问题列表，这里只包含一个问题
// 	m.Question = make([]dns.Question, 1)
// 	// 设置 DNS 查询的问题，包括域名、查询类型（这里是 A 记录）和查询类别（这里是 INET）
// 	m.Question[0] = dns.Question{
// 		Name:   domain,
// 		Qtype:  dns.TypeA,
// 		Qclass: dns.ClassINET,
// 	}
// 	// 返回封装好的 DNS 消息对象和可能的错误
// 	return m, nil
// }

// // 从 DNS 查询中解析封装的数据
// // 该函数用于从接收到的 DNS 查询消息中提取并解码之前封装的其他协议数据
// func extractDataFromDNSQuery(m *dns.Msg) ([]byte, error) {
// 	// 检查 DNS 消息中是否包含查询问题
// 	if len(m.Question) == 0 {
// 		// 如果没有查询问题，返回错误
// 		return nil, errors.New("no question in DNS message")
// 	}
// 	// 获取 DNS 查询问题中的域名
// 	domain := m.Question[0].Name
// 	// 检查域名是否以特定前缀 "encapsulated." 开头
// 	if !strings.HasPrefix(domain, "encapsulated.") {
// 		// 如果不是以该前缀开头，说明不是封装的数据，返回 nil 和 nil 表示未找到封装数据
// 		return nil, nil
// 	}
// 	// 去除域名的前缀和后缀，得到 Base64 编码的数据
// 	encodedData := strings.TrimPrefix(domain, "encapsulated.")
// 	encodedData = strings.TrimSuffix(encodedData, ".example.com.")
// 	// 对 Base64 编码的数据进行解码，得到原始的其他协议数据
// 	return base64.StdEncoding.DecodeString(encodedData)
// }
// func (h *dnsHandler) dumpMsgHeader(m *dns.Msg) string {
// 	buf := new(bytes.Buffer)
// 	buf.WriteString(m.MsgHdr.String() + " ")
// 	buf.WriteString("QUERY: " + strconv.Itoa(len(m.Question)) + ", ")
// 	buf.WriteString("ANSWER: " + strconv.Itoa(len(m.Answer)) + ", ")
// 	buf.WriteString("AUTHORITY: " + strconv.Itoa(len(m.Ns)) + ", ")
// 	buf.WriteString("ADDITIONAL: " + strconv.Itoa(len(m.Extra)))
// 	return buf.String()
// }
