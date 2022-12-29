package server

import (
	"MemTable/db"
	"MemTable/db/cmd"
	"MemTable/logger"
	"MemTable/resp"
	"net"
	"time"
)

type Server struct {
	dbs      []*db.DataBase // 多个可以用于切换的数据库
	dbNum    int            //数据库数量
	clis     *ClientList    // 客户端列表
	tl       *TimeEventList // 事件链表
	url      string         // 监听 url
	commands chan *Client   // 用于解析完毕的协程同步
}

func NewServer(url string) *Server {
	n := 2

	d := make([]*db.DataBase, n)
	for i := 0; i < n; i++ {
		d[i] = db.NewDataBase()
	}

	return &Server{
		dbs:      d,
		dbNum:    n,
		clis:     NewClientList(),
		tl:       NewTimeEventList(),
		url:      url,
		commands: make(chan *Client, 1000),
	}
}

func (s *Server) handleRead(conn net.Conn) {
	//data := make([]byte, 1000)
	client := NewClient(conn, s.dbs[0])

	ch := resp.ParseStream(conn)

	// 这里会阻塞等待有数据到达
	running := true
	for running {

		select {
		// 等待是否有新消息到达
		case parsed := <-ch:

			if parsed.Err != nil {

				if e := parsed.Err.Error(); e == "EOF" {
					logger.Debug("Client", client.id, "Peer ShutDown Connection")
				} else {
					logger.Debug("Client", client.id, "Read Error:", e)
				}
				running = false
				break
			}

			array, ok := parsed.Data.(*resp.ArrayData)
			if !ok {
				logger.Warning("Client", client.id, "Parse Command Error")
				running = false
				break
			}

			client.cmd = array.ToCommand()
			// 如果解析完毕有可以执行的命令，则发送给主线程执行
			//client.cmd = string(data[0:i])
			s.commands <- client

		case r := <-client.res: // fixme : 这里的分支会导致客户端消息乱序吗

			// 将主线程的返回值写入到 socket 中
			_, err := conn.Write([]byte(r))

			if err != nil {
				logger.Warning("Client", client.id, "Write Error")
				running = false
				break
			}

		case <-client.exit:
			running = false
			break
		}
	}

	// 如果是读写发生错误，需要通知事件循环来关闭连接
	if client.status != EXIT {
		// 说明这是异常退出的
		client.status = ERROR
		client.cmd = nil

		// 通知顶层
		s.commands <- client
	}

	err := conn.Close()
	if err != nil {
		return
	}

	logger.Debug("Goroutine Exit")

}

func (s *Server) eventLoop() {

	// 每 300 秒清理一次过期客户端
	s.tl.AddTimeEvent(NewPeriodTimeEvent(func() {
		logger.Debug("TimeEvent: Remove Inactive Clients")
		s.clis.RemoveLongNotUsed(1, 300*time.Second)
	}, time.Now().Add(300*time.Second).Unix(), 300*time.Second,
	))

	for {
		timer := time.NewTimer(time.Second)
		select {
		case <-timer.C:
			logger.Debug("EventLoop: Timer trigger")
			// 需要完成定时任务
			s.tl.ExecuteOneIfExpire()

		case cli := <-s.commands:
			logger.Debug("EventLoop: New Event From Client", cli.id.String())

			if cli.cmd == nil {
				continue
			}
			//println(cli.cmd)

			// 底层发生异常，需要关闭客户端，或者客户端已经关闭了，那么就不处理请求了
			if cli.status == ERROR || cli.status == EXIT {
				// 释放客户端资源
				//delete(UUIDSet,cli.id)
				logger.Debug("EventLoop: Remove Closed Client", cli.id.String())
				s.clis.RemoveClient(cli)
				continue
			}

			// 用于判断是否为新连接
			ok := s.clis.AddClientIfNotExist(cli)

			// 如果是新连接
			if ok {
				logger.Debug("EventLoop: New Client")
			}
			//_, exist := UUIDSet[cli.id]
			//
			//if exist {
			//	println("this is an old client")
			//} else {
			//	println("this is a new client")
			//	UUIDSet[cli.id] = struct{}{}
			//	// 变更为正常状态
			//	cli.status = CONNECTED
			//}

			// 更新时间戳
			//cli.tp = time.Now()
			cli.UpdateTimestamp()
			// 执行命令

			var res resp.RedisData
			if len(cli.cmd) == 2 {
				res = cmd.Get(cli.db, cli.cmd)
			} else {
				res = cmd.Set(cli.db, cli.cmd)
			}

			// fixme: 现在默认是一个空命令
			//res := resp.MakeErrorData("error: unsupported command")

			// 写入回包
			cli.res <- string(res.ToBytes()) // fixme : 这里有阻塞的风险

		}
	}

}

func backgroundLoop() {

	// 完成后台的任务
}

func (s *Server) Start() {

	err := logger.Init("/Users/tangrenchu/GolandProjects/MemTable/logs", "bin.log", logger.DEBUG)
	if err != nil {
		return
	}

	listener, err := net.Listen("tcp", "127.0.0.1:6379")
	if err != nil {
		return
	}

	go s.eventLoop()

	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		go s.handleRead(conn)

	}
}