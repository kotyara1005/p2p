package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

func getMyIp() *net.IPAddr {
	name, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	addr, err := net.ResolveIPAddr("ip", name)

	if err != nil {
		log.Fatal(err)
	}
	return addr
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func GetLocalIPs(ipnet *net.IPNet) (r []net.IP) {
	ip := ipnet.IP
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		if !ip.Equal(ipnet.IP) {
			dup := make(net.IP, len(ip))
			copy(dup, ip)
			r = append(r, dup)
		}
	}
	return r
}

type App struct {
	Id              string
	IP              net.IP
	Port            string
	Interval        time.Duration
	connections     []net.Conn
	connectionsLock sync.Mutex
	sequence        uint64
	sequenceLock    sync.Mutex
	waitGroup       sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
}

func (app *App) add(conn net.Conn) {
	app.connectionsLock.Lock()
	defer app.connectionsLock.Unlock()
	app.connections = append(app.connections, conn)
}

func (app *App) Stop() {
	app.cancel()
	app.waitGroup.Wait()
	for _, conn := range app.connections {
		if conn != nil {
			conn.Close()
		}
	}
}

func (app *App) GetSeq() uint64 {
	app.sequenceLock.Lock()
	defer app.sequenceLock.Unlock()

	app.sequence += 1
	return app.sequence
}

func (app *App) Connect(ip *net.IP) error {
	log.Println("Trying connect to", ip)
	conn, err := net.DialTimeout("tcp", ip.String()+":"+app.Port, time.Second)
	if err != nil {
		log.Println("Connection error", err)
		return err
	}
	app.add(conn)
	app.waitGroup.Add(1)
	go app.handleConn(conn)
	log.Println("Connected to", ip)
	return nil
}

func (app *App) handleConn(conn net.Conn) {
	defer app.waitGroup.Done()
	for {
		select {
		case <-app.ctx.Done():
			return
		default:
		}

		buf := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		bufLen, err := conn.Read(buf)
		if err != nil {
			if !err.(net.Error).Temporary() {
				log.Println("Receive from", conn.RemoteAddr(), err)
				return
			}
			continue
		}
		answer := string(buf[:bufLen])
		log.Println("Receive from", conn.RemoteAddr(), answer)
	}
}

func (app *App) Listen() {
	defer app.waitGroup.Done()
	log.Println("Listen")
	port, _ := strconv.Atoi(app.Port)
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: app.IP, Port: port})
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		select {
		case <-app.ctx.Done():
			return
		default:
		}

		err := l.SetDeadline(time.Now().Add(time.Second))
		if err != nil {
			log.Println(err)
			continue
		}
		conn, err := l.Accept()
		if err != nil {
			if !err.(net.Error).Temporary() {
				log.Fatal(err)
			}
			continue
		}
		app.add(conn)
		app.waitGroup.Add(1)
		go app.handleConn(conn)
	}
}

func (app *App) Send() {
	defer app.waitGroup.Done()
	ticker := time.NewTicker(app.Interval * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			log.Printf("Starting to send messages. Connetions count: %v.", len(app.connections))
			for _, conn := range app.connections {
				seq := app.GetSeq()
				_, err := conn.Write([]byte(app.Id + " " + strconv.FormatUint(seq, 10)))
				log.Printf("Sending to %v. Error: %v. Sequence number: %v.", conn.RemoteAddr(), err, seq)
			}
		}
	}
}

func (app *App) Serve() {
	app.ctx, app.cancel = context.WithCancel(context.Background())
	app.waitGroup.Add(1)
	go app.Listen()
	app.waitGroup.Add(1)
	go app.Send()
}

func main() {
	id := flag.String("id", "", "Application id")
	ip := flag.String("ip", "", "Application ip")
	port := flag.String("port", "53535", "Port for serving incoming notifications")
	mask := flag.String("mask", "29", "Network mask")
	interval := flag.Int("interval", 3, "Interval between sending messages")
	flag.Parse()

	myIp := getMyIp()
	if *ip == "" {
		*ip = myIp.String()
	}
	if *id == "" {
		*id = *ip
	}
	log.SetPrefix(myIp.String() + " ")
	_, ipnet, _ := net.ParseCIDR(*ip + "/" + *mask)
	ips := GetLocalIPs(ipnet)
	app := &App{Id: *id, IP: myIp.IP, Port: *port, Interval: time.Duration(*interval)}
	defer app.Stop()

	app.Serve()
	for _, ip := range ips {
		if ip.Equal(myIp.IP) {
			continue
		}
		buf := make(net.IP, len(ip))
		copy(buf, ip)
		go app.Connect(&buf)
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)
}
