package process

import (
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/mattn/go-shellwords"
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
	s := serverInstance{config: &p, process: make(map[string]*processInstances)}
	a.Nil(s.initLoad())
	// s.startAll()
	m := jsonpb.Marshaler{EmitDefaults: true}
	r := s.readStatusAll()
	_, err = m.MarshalToString(&r)
	a.Nil(err)
}

type testStruct struct {
	a string
}

func TestStructCopy(t *testing.T) {
	a := assert.New(t)
	aa := testStruct{a: "aa"}
	bb := &aa
	var cc testStruct = *bb
	a.Equal(cc.a, "aa")
	cc.a = "cc"
	a.Equal(cc.a, "cc")
	a.Equal(aa.a, "aa")
	aa.a = "aaa"
	a.Equal(aa.a, "aaa")
	a.Equal(bb.a, "aaa")
}

func TestShellwordsParse(t *testing.T) {
	a := assert.New(t)
	args, err := shellwords.Parse("sh -c 'aa'  ")
	a.Nil(err)
	a.Equal(args, []string{"sh", "-c", "aa"})
}
