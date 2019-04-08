package process

import (
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/mattn/go-shellwords"

	"github.com/pkg/errors"
	pb "github.com/wangkechun/gosupervisor/pkg/proto"
)

const defaultProcessWaitTime = time.Second * 3     // 进程启动3秒之后，认为是成功启动
const defaultProcessStopTimeout = time.Second * 30 // 停止进程最多等待30s

type startMode int

const startByAuto startMode = 1    // supervisord 启动的时候启动
const startByManual startMode = 2  // 手动重启
const startByMonitor startMode = 3 // 被monitor线程重启

type processInstances struct {
	spec         *pb.ProcessSpec
	status       pb.ProcessStatus
	args         []string
	cmd          *exec.Cmd
	lock         sync.RWMutex
	monitorLock  sync.RWMutex // start 和 monitor 过程的锁
	lastExitCode int32
	backoffTimes int32
}

func (p *processInstances) stop() error {
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
	p.status.ProcessDesc = "process stopping"
	p.status.Status = pb.ProcessStatus_STOPPING
	log.Println("interrupt process done", name)
	if err != nil {
		return errors.Wrap(err, "interrupt process")
	}
	for i := 0; i < 100; i++ {
		time.Sleep(defaultProcessStopTimeout / 100)
		if p.cmd != nil && p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
			p.status.ProcessDesc = "process stoped"
			p.status.Status = pb.ProcessStatus_STOPPED
			p.cmd = nil
			log.Println("process stoped")
			return nil
		}
	}
	return errors.New("stop process timeout")
}

func (p *processInstances) kill() error {
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
	p.status.ProcessDesc = "process killed"
	p.status.Status = pb.ProcessStatus_STOPPED
	return nil
}

func (p *processInstances) watchProcess() {
	defer p.monitorLock.Unlock()
	exitCh := make(chan error)
	processWaitTime := defaultProcessWaitTime
	if p.spec.Startsecs != 0 {
		processWaitTime = time.Duration(p.spec.Startsecs) * time.Second
	}

	go func() {
		err := p.cmd.Wait()
		exitCh <- err
	}()
	select {
	case <-exitCh:
		// process exited too quickly
		p.lock.Lock()
		defer p.lock.Unlock()
		if p.status.Status != pb.ProcessStatus_STARTING {
			return
		}
		log.Printf("process start failed, name:%v, exit status:%v", p.spec.ProcessName, p.cmd.ProcessState.String())
		p.lastExitCode = int32(p.cmd.ProcessState.ExitCode())
		p.status.Status = pb.ProcessStatus_BACKOFF
		p.backoffTimes++
		if p.backoffTimes == p.spec.Startretries && p.spec.Startretries > 0 {
			p.status.Status = pb.ProcessStatus_FATAL
		}
		if p.cmd != nil {
			p.status.ProcessDesc = p.cmd.ProcessState.String()
		}
	case <-time.After(processWaitTime):
		p.backoffTimes = 0
		log.Println("process alive 3s", p.spec.ProcessName)
		{
			p.lock.Lock()
			p.status.Status = pb.ProcessStatus_RUNNING
			p.status.ProcessDesc = "ok"
			p.lock.Unlock()
		}

		{
			err := <-exitCh
			p.lock.Lock()
			defer p.lock.Unlock()
			if p.status.Status == pb.ProcessStatus_STOPPING || p.status.Status == pb.ProcessStatus_STOPPED {
				return
			}
			p.lastExitCode = int32(p.cmd.ProcessState.ExitCode())
			p.status.Status = pb.ProcessStatus_EXITED
			if p.cmd != nil {
				p.status.ProcessDesc = p.cmd.ProcessState.String()
			}
			log.Println("process exit", p.spec.ProcessName, err)
		}
	}
}

func (p *processInstances) start(mode startMode) error {
	p.monitorLock.Lock()
	shouldUnlockMonitor := true
	defer func() {
		if shouldUnlockMonitor {
			p.monitorLock.Unlock()
		}
	}()
	p.lock.Lock()
	defer p.lock.Unlock()

	switch mode {
	case startByAuto:
		if !p.spec.Autostart {
			return nil
		}
	case startByManual:
		p.backoffTimes = 0
	case startByMonitor:
		shouldContinue := false
		if p.status.Status == pb.ProcessStatus_BACKOFF {
			if p.backoffTimes > p.spec.Startretries && p.spec.Startretries > 0 {
				panic("process shoud not backoff state")
			}
			shouldContinue = true
		}
		if p.status.Status == pb.ProcessStatus_EXITED {
			switch p.spec.Autorestart {
			case pb.ProcessSpec_FALSE:
				shouldContinue = false
			case pb.ProcessSpec_TRUE:
				shouldContinue = true
			case pb.ProcessSpec_UNEXPECTED:
				hasMatched := false
				for _, v := range p.spec.Exitcodes {
					if v == p.lastExitCode {
						hasMatched = true
						break
					}
				}
				if hasMatched {
					shouldContinue = false
				} else {
					shouldContinue = true
				}
			}
		}
		if !shouldContinue {
			return nil
		}
	}

	if p.cmd != nil {
		if p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited() {
			return errors.New("process alread started")
		}
	}
	log.Println("exec.CommandContext", p.spec.ProcessName, p.args)
	p.cmd = exec.Command(p.args[0], p.args[1:]...)
	p.cmd.Stderr = os.Stderr
	p.cmd.Stdout = os.Stdout
	p.cmd.Dir = p.spec.Directory
	p.cmd.Env = p.spec.Environment
	p.status.Status = pb.ProcessStatus_STARTING
	p.status.ProcessDesc = "starting"
	err := p.cmd.Start()
	if err != nil {
		p.status.Status = pb.ProcessStatus_BACKOFF
		p.backoffTimes++
		if p.backoffTimes == p.spec.Startretries && p.spec.Startretries > 0 {
			p.status.Status = pb.ProcessStatus_FATAL
		}
		p.status.ProcessDesc = "start failed " + err.Error()
		return errors.Wrap(err, "process start")
	}
	shouldUnlockMonitor = false
	go p.watchProcess()
	return nil
}

func (p *processInstances) readStatus() pb.Process {
	p.lock.RLock()
	defer p.lock.RUnlock()
	var spec = *p.spec
	status := p.status
	if p.cmd != nil {
		status.Pid = int32(p.cmd.Process.Pid)
	}
	return pb.Process{Spec: &spec, Status: &status}
}

func newProcessInstances(spec *pb.ProcessSpec) (*processInstances, error) {
	p := &processInstances{spec: spec, status: pb.ProcessStatus{}}
	args, err := shellwords.Parse(spec.Command)
	if err != nil {
		return nil, errors.Errorf("shell command is incorrect, name:%v err:%v command:%v", spec.ProcessName, err, spec.Command)
	}
	if len(args) == 0 {
		return nil, errors.Errorf("shell command empty")
	}
	p.args = args
	return p, nil
}
