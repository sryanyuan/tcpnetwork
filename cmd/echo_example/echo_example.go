package main

import (
	"log"
	"sync"

	"bufio"
	"os"
	"syscall"

	"os/signal"

	"sync/atomic"

	"github.com/sryanyuan/tcpnetwork"
)

var (
	kServerAddress  = "localhost:14444"
	serverConnected int32
	stopFlag        int32
)

// echo server routine
func echoServer() (*tcpnetwork.TCPNetwork, error) {
	var err error
	server := tcpnetwork.NewTCPNetwork(1024, tcpnetwork.NewStreamProtocol4())
	err = server.Listen(kServerAddress)
	if nil != err {
		return nil, err
	}

	return server, nil
}

func routineEchoServer(server *tcpnetwork.TCPNetwork, wg *sync.WaitGroup, stopCh chan struct{}) {
	defer func() {
		log.Println("server done")
		wg.Done()
	}()

	for {
		select {
		case evt, ok := <-server.GetEventQueue():
			{
				if !ok {
					return
				}

				switch evt.EventType {
				case tcpnetwork.KConnEvent_Connected:
					{
						log.Println("Client ", evt.Conn.GetRemoteAddress(), " connected")
					}
				case tcpnetwork.KConnEvent_Close:
					{
						log.Println("Client ", evt.Conn.GetRemoteAddress(), " disconnected")
					}
				case tcpnetwork.KConnEvent_Data:
					{
						evt.Conn.Send(evt.Data, 0)
					}
				}
			}
		case <-stopCh:
			{
				return
			}
		}
	}
}

// echo client routine
func echoClient() (*tcpnetwork.TCPNetwork, *tcpnetwork.Connection, error) {
	var err error
	client := tcpnetwork.NewTCPNetwork(1024, tcpnetwork.NewStreamProtocol4())
	conn, err := client.Connect(kServerAddress)
	if nil != err {
		return nil, nil, err
	}

	return client, conn, nil
}

func routineEchoClient(client *tcpnetwork.TCPNetwork, wg *sync.WaitGroup, stopCh chan struct{}) {
	defer func() {
		log.Println("client done")
		wg.Done()
	}()

EVENTLOOP:
	for {
		select {
		case evt, ok := <-client.GetEventQueue():
			{
				if !ok {
					return
				}
				switch evt.EventType {
				case tcpnetwork.KConnEvent_Connected:
					{
						log.Println("Press any thing")
						atomic.StoreInt32(&serverConnected, 1)
					}
				case tcpnetwork.KConnEvent_Close:
					{
						log.Println("Disconnected from server")
						atomic.StoreInt32(&serverConnected, 0)
						break EVENTLOOP
					}
				case tcpnetwork.KConnEvent_Data:
					{
						text := string(evt.Data)
						log.Println(evt.Conn.GetRemoteAddress(), ":", text)
					}
				}
			}
		case <-stopCh:
			{
				return
			}
		}
	}
}

func routineInput(wg *sync.WaitGroup, clientConn *tcpnetwork.Connection) {
	defer func() {
		log.Println("input done")
		wg.Done()
	}()

	reader := bufio.NewReader(os.Stdin)
	for {
		line, _, _ := reader.ReadLine()
		if atomic.LoadInt32(&stopFlag) != 0 {
			return
		}
		str := string(line)

		if str == "\n" {
			continue
		}

		if atomic.LoadInt32(&serverConnected) != 1 {
			log.Println("Not connected")
			continue
		}

		clientConn.Send([]byte(str), 0)
	}
}

func main() {
	// create server
	server, err := echoServer()
	if nil != err {
		log.Println(err)
		return
	}

	// create client
	client, clientConn, err := echoClient()
	if nil != err {
		log.Println(err)
		return
	}

	stopCh := make(chan struct{})

	// process event
	var wg sync.WaitGroup
	wg.Add(1)
	go routineEchoServer(server, &wg, stopCh)

	wg.Add(1)
	go routineEchoClient(client, &wg, stopCh)

	// input event
	wg.Add(1)
	go routineInput(&wg, clientConn)

	// wait
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

MAINLOOP:
	for {
		select {
		case <-sc:
			{
				//	app cancelled by user , do clean up work
				log.Println("Terminating ...")
				break MAINLOOP
			}
		}
	}

	atomic.StoreInt32(&stopFlag, 1)
	log.Println("Press enter to exit")
	close(stopCh)
	wg.Wait()
	clientConn.Close()
	server.Shutdown()
}
