package process

import (
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/pkg/errors"
	pb "github.com/wangkechun/gosupervisor/pkg/proto"
)

const ProcessWaitTime = time.Second * 3 // 进程启动3秒之后，认为是成功启动

type procesInstances struct {
	spec   pb.ProcessSpec
	status pb.ProcessStatus
	args   []string
	cmd    *exec.Cmd
	lock   sync.RWMutex
}

func (p *procesInstances) stop() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	name := p.spec.ProcessName
	log.Println("interrupt process", name)
	if p.cmd == nil {
		return errors.New("process not exists")
	}
	if p.cmd.ProcessState != nil {
		if p.cmd.ProcessState.Exited() {
			return errors.New("process alread stoped")
		}
	}
	err := p.cmd.Process.Signal(os.Interrupt)
	log.Println("interrupt process done", name)
	if err != nil {
		return errors.Wrap(err, "interrupt process")
	}
	for i := 0; i < 300; i++ {
		time.Sleep(time.Second / 10)
		if p.cmd != nil && p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			p.status.Desc = "process stoped"
			p.status.IsManualStop = true
			p.status.Status = pb.ProcessStatus_STOPED
			p.cmd = nil
			log.Println("process stoped")
			return nil
		}
	}
	return errors.New("stop process timeout")
}

func (p *procesInstances) kill() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.cmd == nil {
		return errors.New("process not exists")
	}
	err := p.cmd.Process.Kill()
	if err != nil {
		return errors.Wrap(err, "kill process")
	}
	p.cmd = nil
	p.status.Desc = "process killed"
	p.status.IsManualStop = true
	p.status.Status = pb.ProcessStatus_STOPED
	return nil
}

func (p *procesInstances) watchProcess() {
	exitCh := make(chan error)
	go func() {
		err := p.cmd.Wait()
		exitCh <- err
	}()
	select {
	case <-exitCh:
		p.lock.Lock()
		defer p.lock.Unlock()
		if p.status.IsManualStop {
			return
		}
		log.Printf("process start failed, name:%v, exit status:%v", p.spec.ProcessName, p.cmd.ProcessState.String())
		p.status.Status = pb.ProcessStatus_FAILED
		if p.cmd != nil {
			p.status.Desc = p.cmd.ProcessState.String()
		}
	case <-time.After(ProcessWaitTime):
		log.Println("process alive 3s", p.spec.ProcessName)
		{
			p.lock.Lock()
			p.status.Status = pb.ProcessStatus_RUNNING
			p.status.Desc = "ok"
			p.lock.Unlock()
		}

		{
			err := <-exitCh
			p.lock.Lock()
			defer p.lock.Unlock()
			if p.status.IsManualStop {
				return
			}
			p.status.Status = pb.ProcessStatus_FAILED
			if p.cmd != nil {
				p.status.Desc = p.cmd.ProcessState.String()
			}
			log.Println("process exit", p.spec.ProcessName, err)
		}
	}
}

func (p *procesInstances) start() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.cmd != nil {
		if p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited() {
			return errors.New("process alread started")
		}
	}
	log.Println("exec.CommandContext", p.args)
	p.cmd = exec.Command(p.args[0], p.args[1:]...)
	p.cmd.Stderr = os.Stderr
	p.cmd.Stdout = os.Stdout
	p.cmd.Dir = p.spec.Directory
	p.cmd.Env = p.spec.Environment
	p.status.Status = pb.ProcessStatus_STARTING
	p.status.Desc = ""
	p.status.IsManualStop = false
	err := p.cmd.Start()
	if err != nil {
		p.status.Status = pb.ProcessStatus_FAILED
		p.status.Desc = err.Error()
		return errors.Wrap(err, "process start")
	}
	go p.watchProcess()
	return nil
}

func (p *procesInstances) readStatus() pb.Process {
	p.lock.RLock()
	defer p.lock.RUnlock()
	spec := p.spec
	status := p.status
	if p.cmd != nil {
		status.Pid = int32(p.cmd.Process.Pid)
	}
	return pb.Process{Spec: &spec, Status: &status}
}
