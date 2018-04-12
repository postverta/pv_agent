package client

import (
	"fmt"
	"github.com/postverta/pv_agent/proto"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"time"
)

func OpenContext(host string, port uint, ports []int, env []string, execConfigRoots []string, image string, accountName string, accountKey string, container string, sourceWorktreeId string, worktreeId string, mountPoint string, autoSaveInterval uint) error {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
	if err != nil {
		return err
	}

	client := proto.NewAgentServiceClient(conn)
	uintPorts := make([]uint32, len(ports))
	for i, port := range ports {
		uintPorts[i] = uint32(port)
	}

	req := &proto.OpenContextReq{
		StorageConfig: &proto.StorageConfig{
			AccountName: accountName,
			AccountKey:  accountKey,
			Container:   container,
		},
		Image:            image,
		SourceWorktreeId: sourceWorktreeId,
		WorktreeId:       worktreeId,
		MountPoint:       mountPoint,
		AutosaveInterval: uint32(autoSaveInterval),
		Ports:            uintPorts,
		Env:              env,
		ExecConfigRoots:  execConfigRoots,
	}

	resp, err := client.OpenContext(context.Background(), req)
	if err != nil {
		return err
	}

	fmt.Printf("ContextId: %s\n", resp.ContextId)
	fmt.Printf("GRPC endpoint: %s\n", resp.GrpcEndpoint)
	for _, portEndpoint := range resp.PortEndpoints {
		fmt.Printf("Port %d => %s\n", portEndpoint.Port, portEndpoint.Endpoint)
	}

	return nil
}

func CloseContext(host string, port uint, contextId string) error {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
	if err != nil {
		return err
	}

	client := proto.NewAgentServiceClient(conn)
	req := &proto.CloseContextReq{
		ContextId: contextId,
	}

	_, err = client.CloseContext(context.Background(), req)
	if err != nil {
		return err
	}

	fmt.Println("Successfully closed the context!")

	return nil
}

func StressTest(host string, port uint, ports []int, env []string, execConfigRoots []string, image string, accountName string, accountKey string, container string, sourceWorktreeId string, worktreeId string, mountPoint string, autoSaveInterval uint) error {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
	if err != nil {
		return err
	}

	client := proto.NewAgentServiceClient(conn)
	uintPorts := make([]uint32, len(ports))
	for i, port := range ports {
		uintPorts[i] = uint32(port)
	}

	openReq := &proto.OpenContextReq{
		StorageConfig: &proto.StorageConfig{
			AccountName: accountName,
			AccountKey:  accountKey,
			Container:   container,
		},
		Image:            image,
		SourceWorktreeId: sourceWorktreeId,
		WorktreeId:       worktreeId,
		MountPoint:       mountPoint,
		AutosaveInterval: uint32(autoSaveInterval),
		Ports:            uintPorts,
		Env:              env,
		ExecConfigRoots:  execConfigRoots,
	}

	for {
		startTime := time.Now()
		openResp, err := client.OpenContext(context.Background(), openReq)
		if err != nil {
			log.Fatalf("Stress test failed. Error:", err)
		}
		log.Printf("Open takes %f seconds\n", time.Now().Sub(startTime).Seconds())

		startTime = time.Now()
		closeReq := &proto.CloseContextReq{
			ContextId: openResp.ContextId,
		}
		_, err = client.CloseContext(context.Background(), closeReq)
		if err != nil {
			log.Fatalf("Stress test failed. Error:", err)
		}
		log.Printf("Close takes %f seconds\n", time.Now().Sub(startTime).Seconds())
	}

	return nil
}
