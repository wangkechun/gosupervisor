package process

import (
	"log"
	"sync"

	"github.com/mattn/go-shellwords"
	"github.com/pkg/errors"
	pb "github.com/wangkechun/gosupervisor/pkg/proto"
)

const ServiceVersion = "v0.1"

type serverInstance struct {
	config  *pb.ConfigFile
	process map[string]*procesInstances
	lock    sync.RWMutex
}

func (s *serverInstance) load() error {
	if s.config.Version != ServiceVersion {
		return errors.Errorf("unknow version %v", s.config.Version)
	}
	for _, v := range s.config.Process {
		name := v.ProcessName
		_, ok := s.process[name]
		if ok {
			return errors.Errorf("find repeat process %v", name)
		}
		p := procesInstances{spec: *v, status: pb.ProcessStatus{}}
		args, err := shellwords.Parse(v.Command)
		if err != nil {
			return errors.Errorf("shell command is incorrect, name:%v err:%v command:%v", name, err, v.Command)
		}
		if len(args) == 0 {
			return errors.Errorf("shell command empty")
		}
		p.args = args
		s.process[name] = &p
	}
	return nil
}

func (s *serverInstance) startAll() {
	for _, v := range s.process {
		go func(v *procesInstances) {
			err := v.start()
			if err != nil {
				log.Printf("start process:%v, err:%v", v.spec.ProcessName, err)
			}
		}(v)
	}
}

func (s *serverInstance) readStatusAll() pb.ListReply {
	s.lock.RLock()
	defer s.lock.RUnlock()
	r := pb.ListReply{}
	for _, v := range s.config.Process {
		name := v.ProcessName
		p := s.process[name]
		st := p.readStatus()
		r.Process = append(r.Process, &st)
	}
	return r
}
