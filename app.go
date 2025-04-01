package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	for {
		fmt.Println("选择要运行的隧道协议：")
		fmt.Println("1. DNS 隧道")
		fmt.Println("2. SMTP 隧道")
		fmt.Println("3. POP3 隧道")
		fmt.Println("4. HTTPS 隧道")
		fmt.Println("5. SSH 隧道")
		fmt.Println("6. QUIC 隧道")
		fmt.Println("输入 q 退出程序")
		fmt.Print("请输入数字选择协议：")

		var choice string
		fmt.Scan(&choice)

		if choice == "q" {
			fmt.Println("程序已退出")
			return
		}

		switch choice {
		case "1":
			runProtocol("dns")
			fmt.Println("---------监听udp观察DNS请求-----------")
		case "2":
			runProtocol("smtp")
			fmt.Println("--------监听tcp==25观察SMTP请求-------")
		case "3":
			runProtocol("pop3")
			fmt.Println("--------监听tcp==110观察POP3请求-------")
		case "4":
			runProtocol("https")
			fmt.Println("--------监听tcp==443观察HTTPS请求-------")
		case "5":
			runProtocol("ssh")
			fmt.Println("--------监听tcp==22观察SSH请求---------")
		case "6":
			runProtocol("quic")
			fmt.Println("--------监听udp==443观察QUIC请求-------")
		default:
			fmt.Println("无效的选择，请重新输入")
		}
	}
}

func runProtocol(protocol string) {
	// 获取当前执行文件的目录
	exPath, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取执行文件路径失败: %v\n", err)
		return
	}
	fmt.Printf("当前执行文件路径: %s\n", exPath)
	// 构造协议程序的完整路径
	protocolDir := filepath.Join(exPath, "AAAtestLS", "wrap"+protocol)

	// 切换到协议程序所在目录
	err = os.Chdir(protocolDir)
	if err != nil {
		fmt.Printf("切换目录失败: %v\n", err)
		return
	}

	// 运行协议程序
	cmd := exec.Command("go", "run", "main.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("运行协议失败: %v\n", err)
	}

	// 切换回原目录
	err = os.Chdir(exPath)
	if err != nil {
		fmt.Printf("切换回原目录失败: %v\n", err)
	}
}
