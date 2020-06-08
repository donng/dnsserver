package main

import (
	"flag"
	"log"
	"net"
	"sync"

	"golang.org/x/net/dns/dnsmessage"
)

type DNSInterface interface {
	Listen(int)
	Query(Packet)
	Forward(dnsmessage.Message)
	Send(dnsmessage.Message, *net.UDPAddr)
}

type DNSService struct {
	conn       *net.UDPConn
	store      *Store
	forwarders map[string][]Packet
}

// 包数据
type Packet struct {
	addr    *net.UDPAddr
	message dnsmessage.Message
}

const (
	Port   = 53
	Length = 512
)

var (
	ttl *int64
	rw sync.RWMutex
)

// 知识点1：根据包 Header 中的 ID 来对应 DNS 的查询和响应
// 知识点2：根据包 Header 中的 Response 判断是 DNS 查询还是转发的响应

// DNS 本地服务器，转发域名解析并缓存服务
// 1. 监听 53 端口
// 2. 解析数据报，如果存在缓存则直接返回。
// 3. 无缓存时，请求加入到队列，并转发 DNS 服务
// 3. 如果是114解析的响应，则缓存并消费队列中的数据

// 端口 53 开启 DNS 服务
// 客户端访问服务： nslookup somewhere.com some.dns.server
// dig @localhost somewhere.com
func main() {
	ttl = flag.Int64("ttl", 60, "DNS缓存过期时间，单位为秒")
	port := flag.Int("p", Port, "服务端口号，默认为53")
	flag.Parse()

	// dns api service
	go startApi()

	s := NewDNSService()
	s.Listen(*port)
}

func NewDNSService() *DNSService {
	dns := DNSService{
		store:      store,
		forwarders: make(map[string][]Packet),
	}
	return &dns
}

func (s *DNSService) Listen(port int) {
	var err error
	s.conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		log.Fatalf("service start failed, error: %s", err)
	}
	defer s.conn.Close()

	log.Printf("dns service start, port: %d \n", port)

	for {
		buf := make([]byte, Length)
		_, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read from udp failed, error: %s \n", err)
			continue
		}

		var m dnsmessage.Message
		if err = m.Unpack(buf); err != nil {
			log.Printf("dmsmessage unpack failed, error: %s \n", err)
			continue
		}
		if len(m.Questions) == 0 {
			continue
		}

		go s.Query(Packet{addr, m})
	}
}

func (s *DNSService) Query(p Packet) {
	domain := p.message.Questions[0].Name.String()

	// check is request or response from forward
	if p.message.Response {
		// cache
		s.store.Set(domain, p.message)
		// get client addr
		for i, v := range s.forwarders[domain] {
			if v.message.ID == p.message.ID {
				go s.Send(p.message, v.addr)
				// remove current client
				if len(s.forwarders[domain])-1 == i {
					s.forwarders[domain] = s.forwarders[domain][:len(s.forwarders[domain])-1]
				} else {
					s.forwarders[domain] = append(s.forwarders[domain][:i], s.forwarders[domain][i+1:]...)
				}
				break
			}
		}
		return
	}

	log.Printf("get request,domain: %s, ip: %s，ID: %d \n", domain, p.addr.IP, p.message.ID)

	// request from client, check cache
	if message, ok := s.store.Get(domain); ok {
		message.ID = p.message.ID
		go s.Send(p.message, p.addr)
	}
	//  if cache not exist, add forwarders and forward
	rw.Lock()
	s.forwarders[domain] = append(s.forwarders[domain], Packet{
		addr:    p.addr,
		message: p.message,
	})
	rw.Unlock()

	s.Forward(p.message)
}

func (s *DNSService) Send(message dnsmessage.Message, addr *net.UDPAddr) {
	packed, err := message.Pack()
	if err != nil {
		log.Printf("dnsmessage pack failed. header ID: %d, error: %s \n", message.ID, err)
		return
	}
	_, err = s.conn.WriteToUDP(packed, addr)
	if err != nil {
		log.Printf("response to client failed, error: %s \n", err)
	}
}

func (s *DNSService) Forward(message dnsmessage.Message) {
	packed, err := message.Pack()
	if err != nil {
		log.Printf("dnsmessage pack failed. header ID: %d, error: %s \n", message.ID, err)
		return
	}

	resolver := net.UDPAddr{IP: net.IP{114, 114, 114, 114}, Port: 53}
	_, err = s.conn.WriteToUDP(packed, &resolver)
	if err != nil {
		log.Printf("response to client failed, error: %s \n", err)
	}
}
