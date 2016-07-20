# tcpnetwork

A simple golang tcp server/client.

# purpose

Provide a simple model to process multi connections.You can use tcpnetwork as a server and as a client, then process all event in a output channel, so you just need one routine (maybe main routine) to process all things.

# protobuf support

You can directly use tcpnetwork to send a protobuf message, and set a hook to unserialize your protobuf message.

# example

*  echo server

		package main
		
		import (
		    "log"
		
		    "github.com/sryanyuan/tcpnetwork"
		)
		
		type TSShutdown struct {
		    network *tcpnetwork.TCPNetwork
		}
		
		func NewTSShutdown() *TSShutdown {
		    t := &TSShutdown{}
		    t.network = tcpnetwork.NewTCPNetwork(1024, tcpnetwork.NewStreamProtocol4())
		    return t
		}
		
		func (this *TSShutdown) OnConnected(evt *tcpnetwork.ConnEvent) {
		    log.Println("connected ", evt.Conn.GetConnId())
		}
		
		func (this *TSShutdown) OnDisconnected(evt *tcpnetwork.ConnEvent) {
		    log.Println("disconnected ", evt.Conn.GetConnId())
		}
		
		func (this *TSShutdown) OnRecv(evt *tcpnetwork.ConnEvent) {
		    log.Println("recv ", evt.Conn.GetConnId(), evt.Data)
		
		    evt.Conn.Send(evt.Data, false)
		}
		
		func (this *TSShutdown) Run() {
		    err := this.network.Listen("127.0.0.1:2222")
		
		    if err != nil {
		        log.Println(err)
		        return
		    }
		
		    this.network.ServeWithHandler(this)
		    log.Println("done")
		}
		
		
		package main
		
		import (
		    "fmt"
		    "log"
		)
		
		func main() {
		    defer func() {
		        e := recover()
		        if e != nil {
		            log.Println(e)
		        }
		        var inp int
		        fmt.Scanln(&inp)
		    }()
		    tsshutdown := NewTSShutdown()
		    tsshutdown.Run()
		}