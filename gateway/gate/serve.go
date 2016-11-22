package gate

import (
	"net"

	proto "github.com/aiyun/gomqtt/mqtt/protocol"
	"github.com/aiyun/gomqtt/mqtt/service"
	"github.com/corego/tools"
	"github.com/uber-go/zap"

	rpc "github.com/aiyun/gomqtt/proto"
)

func serve(c net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			Logger.Info("user's main goroutine has a panic error", zap.Error(err.(error)))
			Logger.Info("stack", zap.Stack())
		}
	}()

	// init a new connInfo
	ci := &connInfo{}

	//generate a uuid for this conn
	ci.id = 1
	ci.c = c
	Logger.Debug("a new connection has established", zap.Int64("cid", ci.id), zap.String("ip", c.RemoteAddr().String()))

	defer func() {
		c.Close()

		// 要保证所有全局结构中引用当前ci的地方，都删除
		delCI(ci.id)

		delMutex(ci)

		close(ci.relogin)
	}()

	ci.stopped = make(chan struct{})
	ci.relogin = make(chan struct{})

	//----------------Connection init---------------------------------------------
	reply := proto.NewConnackPacket()
	err, code := initConnection(ci)
	reply.SetReturnCode(code)
	if err != nil {
		Logger.Info("user connect failed", zap.Int64("cid", ci.id), zap.Error(err), zap.String("user", tools.Bytes2String(ci.cp.Username())), zap.String("password", tools.Bytes2String(ci.cp.Password())))
		service.WritePacket(ci.c, reply)
		return
	}

	if err := service.WritePacket(ci.c, reply); err != nil {
		Logger.Info("write connecaccept failed", zap.Int64("cid", ci.id), zap.Error(err), zap.String("user", tools.Bytes2String(ci.cp.Username())), zap.String("password", tools.Bytes2String(ci.cp.Password())))
		return
	}

	Logger.Debug("user connected ok!", zap.String("user", tools.Bytes2String(ci.cp.Username())), zap.String("password", tools.Bytes2String(ci.cp.Password())), zap.Int64("cid", ci.id),
		zap.Float64("keepalive", float64(ci.cp.KeepAlive())))

	// save ci
	saveCI(ci)

	go recvPacket(ci)

	// loop reading data
	for {
		select {
		case <-ci.stopped:
			Logger.Info("user's main thread is going to stop")
			goto STOP
		}
	}

STOP:
	ci.rpc.logout(&rpc.LogoutMsg{
		An:  ci.cp.Username(),
		Un:  ci.cp.ClientId(),
		Cid: ci.id,
	})
}
