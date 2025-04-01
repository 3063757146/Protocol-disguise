package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	//"os"
	"os/exec"
	"time"
)

type Config struct {
	ServerIP        string `json:"serverIP"`
	ServerPort      int    `json:"serverPort"`
	LocalPort       int    `json:"localPort"`
	ClientLocalPort int    `json:"clientLocalPort"`
}

var (
	serverCmd *exec.Cmd
	clientCmd *exec.Cmd
)

func main() {
	// 读取配置文件
	config := loadConfig("config.json")

	// 启动服务端
	go startServer(config.ServerIP, config.ServerPort, config.LocalPort)
	time.Sleep(2 * time.Second) // 等待服务端启动

	// 启动 客户端
	go startClient(config.ClientLocalPort, config.ServerIP, config.ServerPort)
	time.Sleep(2 * time.Second) // 等待客户端启动

	// 进行 curl 测试
	testCURL(config.ClientLocalPort)

	// 进行文件下载测试
	testFileDownload(config.ClientLocalPort)

	fmt.Println("----------为不影响演示，默认在测试任务结束后杀死客户端和服务端进程------------------")
	cleanup()

	// 防止主goroutine退出
	// select {}
}

func loadConfig(filename string) Config {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		return Config{}
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("解析配置文件失败: %v\n", err)
		return Config{}
	}

	return config
}

func startServer(serverIP string, serverPort, localPort int) {
	// 构建  服务端命令
	serverCmd = exec.Command("gost", "-L", fmt.Sprintf("tls://%s:%d/:%d", serverIP, serverPort, localPort))

	// 打印服务端输出
//	serverCmd.Stdout = os.Stdout
//	serverCmd.Stderr = os.Stderr

	err := serverCmd.Start()
	if err != nil {
		fmt.Printf("启动 服务端失败: %v\n", err)
		return
	}

	// 确保进程启动成功
	if serverCmd.Process == nil {
		fmt.Println("服务端进程未启动")
		return
	}

	// 等待进程退出
	go func() {
		err := serverCmd.Wait()
		if err != nil {
			fmt.Printf(" 服务端退出，错误信息: %v\n", err)
		}
	}()

	fmt.Printf("服务端已启动，监听 %s:%d，转发到本地端口 %d\n", serverIP, serverPort, localPort)
}

func startClient(clientLocalPort int, serverIP string, serverPort int) {
	// 构建 客户端命令
	clientCmd = exec.Command("gost", "-L", fmt.Sprintf("tcp://:%d", clientLocalPort), "-F", fmt.Sprintf("forward+tls://%s:%d?secure=false", serverIP, serverPort))

	// 打印客户端输出
//	clientCmd.Stdout = os.Stdout
//	clientCmd.Stderr = os.Stderr

	err := clientCmd.Start()
	if err != nil {
		fmt.Printf("启动客户端失败: %v\n", err)
		return
	}

	// 确保进程启动成功
	if clientCmd.Process == nil {
		fmt.Println("客户端进程未启动")
		return
	}

	// 等待进程退出
	go func() {
		err := clientCmd.Wait()
		if err != nil {
			fmt.Printf("客户端退出，错误信息: %v\n", err)
		}
	}()

	fmt.Printf("客户端已启动，监听本地端口 %d，转发到 %s:%d\n", clientLocalPort, serverIP, serverPort)
}

func testCURL(clientLocalPort int) {
	fmt.Printf("---------正在测试连接...curl http://localhost:%d\n", clientLocalPort)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d", clientLocalPort))
	if err != nil {
		fmt.Printf("CURL 测试失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 打印响应状态码
	fmt.Printf("-------------CURL 测试成功: 状态码 %d\n", resp.StatusCode)

	// 打印响应头
	fmt.Println("响应头信息:")
	for key, values := range resp.Header {
		fmt.Printf("%s: %v\n", key, values)
	}

	// 打印响应体
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应体失败: %v\n", err)
		return
	}
	fmt.Println("响应体信息:")
	fmt.Println(string(body))
}

func testFileDownload(clientLocalPort int) {
	fmt.Printf("---------正在测试文件下载...http://localhost:%d/test.png\n", clientLocalPort)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/test.png", clientLocalPort))
	if err != nil {
		fmt.Printf("文件下载测试失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 打印响应状态码
	fmt.Printf("文件下载成功响应: 状态码 %d\n", resp.StatusCode)
	// 检查 HTTP 状态码
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fmt.Printf("文件不存在 文件下载测试失败: 状态码 %d\n", resp.StatusCode)
		return
	}
	// 打印响应头
	fmt.Println("响应头信息:")
	for key, values := range resp.Header {
		fmt.Printf("%s: %v\n", key, values)
	}

	// 保存响应体到文件
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应体失败: %v\n", err)
		return
	}

	// 将文件保存到本地
	err = ioutil.WriteFile("downloaded_test.png", body, 0644)
	if err != nil {
		fmt.Printf("保存文件失败: %v\n", err)
		return
	}

	fmt.Println("文件已下载并保存为 downloaded_test.png")
}

func cleanup() {
	// 终止服务端进程
	if serverCmd != nil && serverCmd.Process != nil {
		err := serverCmd.Process.Kill()
		if err != nil {
			fmt.Printf("终止服务端进程时出错: %v\n", err)
		} else {
			fmt.Println("服务端进程已终止")
		}
	}

	// 终止客户端进程
	if clientCmd != nil && clientCmd.Process != nil {
		err := clientCmd.Process.Kill()
		if err != nil {
			fmt.Printf("终止客户端进程时出错: %v\n", err)
		} else {
			fmt.Println("客户端进程已终止")
		}
	}

	fmt.Println("所有子进程已清理，程序退出")
}
