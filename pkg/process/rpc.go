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
		err = p.start(startByManual)
		return &pb.CommandReply{}, err
	case pb.CommandRequest_START:
		err = p.start(startByManual)
		return &pb.CommandReply{}, err
	case pb.CommandRequest_STOP:
		err = p.stop()
		return &pb.CommandReply{}, err
	}
	return nil, status.Error(codes.Unimplemented, "Unimplemented")
}

func RunServer(cfgPath string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	buf, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return errors.Wrap(err, "read config failed")
	}
	var p pb.ConfigFile
	err = proto.UnmarshalText(string(buf), &p)
	if err != nil {
		return errors.Wrap(err, "parse config failed")
	}
	s := &serverInstance{config: &p, process: make(map[string]*processInstances)}
	err = s.initLoad()
	if err != nil {
		return errors.Wrap(err, "load config failed")
	}
	s.initStartAll()
	go s.initRunMonitor(ctx)
	lis, err := net.Listen("tcp", p.RpcAddr)
	if err != nil {
		return errors.Wrapf(err, "failed to listen, addr %v", p.RpcAddr)
	}
	svr := grpc.NewServer()
	pb.RegisterGoSupervisorServer(svr, s)
	log.Println("gosupervisor listen addr:", p.RpcAddr)
	if err := svr.Serve(lis); err != nil {
		return errors.Wrap(err, "failed to serve")
	}
	return nil
}
