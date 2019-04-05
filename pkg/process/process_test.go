package process

import (
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	pb "github.com/wangkechun/gosupervisor/pkg/proto"
)

func TestOk(t *testing.T) {
	a := assert.New(t)
	config := `
version:"v0.1"
rpc_addr:"127.0.0.1:7766"
process:{
	process_name:"sleep_1s"
	command:"sleep 1"
	directory:"/tmp"
	environment:"HELLO=WORLD"
	environment:"VERSION=1"
}

process:{
	process_name:"sleep_60s"
	command:"sleep 60"
	directory:"/tmp"
	environment:"HELLO=WORLD"
	environment:"VERSION=1"
}

process:{
	process_name:"ps"
	command:"ps"
	directory:"/tmp"
	environment:"HELLO=WORLD"
	environment:"VERSION=1"
}
	`
	var p pb.ConfigFile
	err := proto.UnmarshalText(config, &p)
	a.Nil(err)
	s := serverInstance{config: &p, process: make(map[string]*procesInstances)}
	a.Nil(s.load())
	// s.startAll()
	m := jsonpb.Marshaler{EmitDefaults: true}
	r := s.readStatusAll()
	_, err = m.MarshalToString(&r)
	a.Nil(err)
}
