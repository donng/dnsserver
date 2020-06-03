package main

import (
	"fmt"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"sync"
)

// 存储响应
type Store struct {
	sync.RWMutex
	data map[string][]dnsmessage.Resource
}

// 包数据
type Packet struct {
	addr    *net.UDPAddr
	message dnsmessage.Message
}

var (
	rw       sync.RWMutex
	conn     *net.UDPConn
	messages = make(map[string][]Packet)
)

// 知识点1：根据包 Header 中的 ID 来对应 DNS 的查询和响应
// 知识点2：根据包 Header 中的 Response 判断是 DNS 查询还是转发的响应

// DNS 本地服务器，转发域名解析并缓存服务
// 1. 监听 53 端口
// 2. 解析数据报，如果存在缓存则直接返回。
// 3. 无缓存时，查看数据报的结果数据，无结果说明是解析请求，需要加入到请求队列，并转发 DNS 服务
// 3. 有结果说明是114请求，缓存请求数据，循环请求队列，服务条件的触发响应返回

// 端口 53 开启 DNS 服务
// 客户端访问服务： nslookup somewhere.com some.dns.server
// dig @localhost somewhere.com
func main() {
	var err error
	conn, err = net.ListenUDP("udp", &net.UDPAddr{
		Port: 53,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	for {
		buf := make([]byte, 512)
		// 通过conn读取UDP报文，将数据填充到buf中
		_, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println(err)
			continue
		}

		// 解析包并判断是否是DNS消息
		var m dnsmessage.Message
		if err = m.Unpack(buf); err != nil {
			log.Println(err)
			continue
		}
		fmt.Printf("%+v \n", m)
		// 无请求信息，返回
		if len(m.Questions) == 0 {
			continue
		}

		go query(Packet{remoteAddr, m})
	}
}

func query(p Packet) {
	domain := p.message.Questions[0].Name.String()

	// 通过 response 区分是客户端请求还是转发的响应
	if p.message.Response {
		sendPacket(domain, p)
		return
	}

	packed, err := p.message.Pack()
	if err != nil {
		fmt.Printf("packet pack err: %s \n", err)
		return
	}
	// 添加到队列中
	rw.Lock()
	messages[domain] = append(messages[domain], p)
	rw.Unlock()

	// 转发请求
	resolver := net.UDPAddr{IP: net.IP{114, 114, 114, 114}, Port: 53}
	_, err = conn.WriteToUDP(packed, &resolver)
}

func sendPacket(domain string, p Packet) {
	// 获得需要响应的数据
	rw.Lock()
	packets := messages[domain]
	for i, packet := range packets {
		if p.message.Header.ID == packet.message.Header.ID {
			// 删除当前元素
			if len(messages[domain])-1 == i {
				messages[domain] = messages[domain][:len(packets)-1]
			} else {
				messages[domain] = append(messages[domain][:i], messages[domain][i+1:]...)
			}
			// 压缩数据包
			packed, err := p.message.Pack()
			if err != nil {
				fmt.Println(err)
			}
			if _, err := conn.WriteToUDP(packed, packet.addr); err != nil {
				fmt.Printf("响应错误 err: %s \n", err)
			}
			break
		}
	}
	rw.Unlock()

	fmt.Printf("队列长度：%d", len(messages[domain]))
}
