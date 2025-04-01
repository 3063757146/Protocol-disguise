import socket  
import signal  
import sys  
import threading  

# 创建一个UDP套接字  
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)  
sock.bind(('127.0.0.1', 8001))  

print("UDP服务器已启动，正在监听端口8001...")  

# 创建一个标志变量来控制服务器的运行状态  
running = True  

def listen_for_input():  
    global running  
    while running:  
        user_input = input()  
        if user_input.lower() == 'q':  
            print("\n正在关闭UDP服务器...")  
            running = False  
            sock.close()  
            break  

# 启动一个线程侦听用户输入  
input_thread = threading.Thread(target=listen_for_input)  
input_thread.start()  

while running:  
    try:  
        data, addr = sock.recvfrom(1024)  # 缓冲区大小为1024字节  
        print(f"收到数据: {data.decode()} 来自: {addr}")  

        # 数据处理逻辑  
        response_data = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n"        
        sock.sendto(response_data.encode(), addr)  
        print(f"发送响应: {response_data} 至: {addr}")  
    except Exception as e:  
        print(f"发生错误: {e}")  

# 确保在退出前等待输入线程结束  
input_thread.join()  
print("UDP服务器已关闭。")  