generate:
	@protoc -I pkg/proto --go_out=plugins=grpc:pkg/proto  gosupervisor.proto
