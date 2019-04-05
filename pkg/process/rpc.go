package process

import (
	"context"
	"io/ioutil"
	"log"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	pb "github.com/wangkechun/gosupervisor/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *serverInstance) Ping(ctx context.Context, req *pb.PingRequest) (resp *pb.PingReply, err error) {
	return &pb.PingReply{ServiceVersion: ServiceVersion}, nil
}

func (s *serverInstance) List(ctx context.Context, req *pb.ListRequest) (resp *pb.ListReply, err error) {
	r := s.readStatusAll()
	return &r, nil
}

func (s *serverInstance) Command(ctx context.Context, req *pb.CommandRequest) (resp *pb.CommandReply, err error) {
	name := req.ProcessName
	cmd := req.Command
	s.lock.Lock()
	p, ok := s.process[name]
	s.lock.Unlock()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "process %v not found", name)
	}
	switch cmd {
	case pb.CommandRequest_KILL:
		err := p.kill()
		return &pb.CommandReply{}, err
	case pb.CommandRequest_RESTART:
		err = p.stop()
		if err != nil {
			return &pb.CommandReply{}, err
		}
		err = p.start()
		return &pb.CommandReply{}, err
	case pb.CommandRequest_START:
		err = p.start()
		return &pb.CommandReply{}, err
	case pb.CommandRequest_STOP:
		err = p.stop()
		return &pb.CommandReply{}, err
	}
	return nil, status.Error(codes.Unimplemented, "未实现")
}

func RunServer(cfgPath string) error {
	buf, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return errors.Wrap(err, "read config failed")
	}
	var p pb.ConfigFile
	err = proto.UnmarshalText(string(buf), &p)
	if err != nil {
		return errors.Wrap(err, "parse config failed")
	}
	s := &serverInstance{config: &p, process: make(map[string]*procesInstances)}
	err = s.load()
	if err != nil {
		return errors.Wrap(err, "load config failed")
	}
	s.startAll()

	lis, err := net.Listen("tcp", p.RpcAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to listen, addr %v", p.RpcAddr)
	}
	svr := grpc.NewServer()
	pb.RegisterGoSupervisorServer(svr, s)
	if err := svr.Serve(lis); err != nil {
		return errors.Wrap(err, "failed to serve")
	}
	log.Println("gosupervisor start success, addr:", p.RpcAddr)
	return nil
}

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}
