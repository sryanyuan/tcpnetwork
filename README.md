# tcpnetwork

A simple golang tcp server/client.

# purpose

Provide a simple model to process multi connections.You can use tcpnetwork as a server and as a client, then process all event in a output channel, so you just need one routine (maybe main routine) to process all things.

# protobuf support

You can directly use tcpnetwork to send a protobuf message, and set a hook to unserialize your protobuf message.

# asynchoronously process event

A simple way to process connection event is to process all event in one routine by reading the event channel tcpnetwork.GetEventQueue(). 

	server := tcpnetwork.NewTCPNetwork(1024, tcpnetwork.NewStreamProtocol4())
	err = :server.Listen(kServerAddress)
	if nil != err {
		return panic(err)
	}

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

# synchronously process event

When you need to process event in every connection routine, you can set a synchronously process function callback by setting the Connection's SetSyncExecuteFunc.

Returning true in this callback function represents the processed event will not be put in the event queue.

	client := tcpnetwork.NewTCPNetwork(1024, tcpnetwork.NewStreamProtocol4())
	conn, err := client.Connect(kServerAddress)
	conn.SetSyncExecuteFunc(...)

# example

*  echo server

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
		
* echo client
		
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

See more example in cmd directory.

# license

MIT