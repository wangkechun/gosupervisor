package process

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"
	pb "github.com/wangkechun/gosupervisor/pkg/proto"
)

const ServiceVersion = "v0.1"
const monitorInterval = time.Second

type serverInstance struct {
	config  *pb.ConfigFile
	process map[string]*processInstances
	lock    sync.RWMutex
}

func (s *serverInstance) initLoad() error {
	if s.config.Version != ServiceVersion {
		return errors.Errorf("unknow version %v", s.config.Version)
	}
	for _, v := range s.config.Process {
		name := v.ProcessName
		_, ok := s.process[name]
		if ok {
			return errors.Errorf("find duplicate process %v", name)
		}
		p, err := newProcessInstances(v)
		if err != nil {
			return errors.Wrap(err, "newProcessInstances")
		}
		s.process[name] = p
	}
	return nil
}

func (s *serverInstance) initStartAll() {
	for _, v := range s.process {
		go func(v *processInstances) {
			err := v.start(startByAuto)
			if err != nil {
				log.Printf("start process:%v, err:%v", v.spec.ProcessName, err)
			}
		}(v)
	}
}

func (s *serverInstance) monitor() {
	s.lock.RLock()
	process := make([]*processInstances, 0)
	for _, v := range s.process {
		status := v.readStatus()
		if status.Status.Status == pb.ProcessStatus_BACKOFF || status.Status.Status == pb.ProcessStatus_EXITED {
			process = append(process, v)
		}
	}
	s.lock.RUnlock()

	for _, process := range process {
		log.Println("monitor try start process", process.spec.ProcessName)
		err := process.start(startByMonitor)
		if err != nil {
			log.Println("monitor try start process failed", process.spec.ProcessName, err)
		}
	}
}

func (s *serverInstance) initRunMonitor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.monitor()
		time.Sleep(monitorInterval)
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
