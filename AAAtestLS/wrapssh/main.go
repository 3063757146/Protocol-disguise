package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
//	"os"
	"os/exec"
	"time"
)

type Config struct {
	Server struct {
		SSHPort    int `json:"ssh_port"`    // 服务端SSH隧道监听端口
		TargetPort int `json:"target_port"` // 服务端目标端口（如HTTP服务端口）
	} `json:"server"`
	Client struct {
		LocalPort     int    `json:"local_port"`      // 客户端本地监听端口
		ServerIP      string `json:"server_ip"`       // 服务端IP地址
		ServerSSHPort int    `json:"server_ssh_port"` // 服务端SSH隧道端口
	} `json:"client"`
}

var (
	serverCmd *exec.Cmd
	clientCmd *exec.Cmd
)

func main() {
	// 读取配置文件
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		return
	}

	// 启动服务端
	go startServer(config.Server.SSHPort, config.Server.TargetPort)

	// 启动客户端
	go startClient(config.Client.LocalPort, config.Client.ServerIP, config.Client.ServerSSHPort)

	// 等待几秒钟确保服务已启动
	time.Sleep(5 * time.Second)

	// 运行自动化测试
	runTests(config.Client.LocalPort)
	testFileDownload(config.Client.LocalPort)
	// 测试结束后，清理子进程
	fmt.Println("为不影响演示 默认上面测试task结束后杀死客户端服务端进程")
	cleanup()

	// // 防止主goroutine退出
	// select {}
}

func loadConfig(filename string) (Config, error) {
	data, err := ioutil.ReadFile(filename)
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

func startServer(sshPort, targetPort int) {
	// 服务端命令：gost -L ssh://:[ssh_port]/:[target_port]
	serverCmd = exec.Command("gost", "-L", fmt.Sprintf("ssh://:%d/:%d", sshPort, targetPort))

	// 打印服务端输出
//	serverCmd.Stdout = os.Stdout
//	serverCmd.Stderr = os.Stderr

	err := serverCmd.Start()
	if err != nil {
		fmt.Printf("启动服务端失败: %v\n", err)
		return
	}
	fmt.Printf("服务端已启动，监听 SSH 端口: %d，转发到目标端口: %d\n", sshPort, targetPort)
}

func startClient(localPort int, serverIP string, serverSSHPort int) {
	// 客户端命令：gost -L tcp://127.0.0.1:[local_port] -F forward+ssh://[server_ip]:[server_ssh_port]
	clientCmd = exec.Command("gost", "-L", fmt.Sprintf("tcp://127.0.0.1:%d", localPort), "-F", fmt.Sprintf("forward+ssh://%s:%d", serverIP, serverSSHPort))

	// 打印客户端输出
//	clientCmd.Stdout = os.Stdout
//	clientCmd.Stderr = os.Stderr

	err := clientCmd.Start()
	if err != nil {
		fmt.Printf("启动客户端失败: %v\n", err)
		return
	}
	fmt.Printf("客户端已启动，监听本地端口: %d，转发到服务端 %s:%d\n", localPort, serverIP, serverSSHPort)
}

func runTests(localPort int) {
	// 发送测试请求
	fmt.Printf("开始自动化测试...\n")

	// 测试HTTP请求
	fmt.Printf("-------------测试HTTP请求: http://127.0.0.1:%d\n", localPort)
	cmd := exec.Command("curl", "-v", fmt.Sprintf("http://127.0.0.1:%d", localPort))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("测试失败: %v\n", err)
	} else {
		fmt.Printf("测试成功，响应:\n%s\n", output)
	}
}
func testFileDownload(clientLocalPort int) {
	fmt.Printf("------------正在测试文件下载...http://localhost:%d/test.png\n", clientLocalPort)

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
