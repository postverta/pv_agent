package server

import (
	"fmt"
	"github.com/postverta/pv_agent/proto"
	"google.golang.org/grpc"
	"log"
	"net"
)

func Start(host string, port uint, execBinaryPath string) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	server, err := NewAgentServer(host, execBinaryPath)
	if err != nil {
		return err
	}

	log.Printf("Server listening on %s:%d\n", host, port)
	proto.RegisterAgentServiceServer(grpcServer, server)
	grpcServer.Serve(lis)
	return nil
}
