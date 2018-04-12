package server

import (
	"errors"
	"fmt"
	"github.com/postverta/pv_agent/proto"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"
)

type AgentServer struct {
	HostIp         string
	ExecBinaryPath string
	dockerClient   *dockerclient.Client
}

func NewAgentServer(hostIp string, execBinaryPath string) (*AgentServer, error) {
	cli, err := dockerclient.NewEnvClient()
	if err != nil {
		return nil, err
	}

	s := &AgentServer{
		HostIp:         hostIp,
		ExecBinaryPath: execBinaryPath,
		dockerClient:   cli,
	}

	err = s.Cleanup()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *AgentServer) Cleanup() error {
	// Delete all running containers and RBD mounts
	containers, err := s.dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, container := range containers {
		// Delete the containers (by force)
		s.dockerClient.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{Force: true})
	}

	// Check whether there is still any run away mount
	mountCmd := exec.Command("mount")
	mountOutput, err := mountCmd.Output()
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(mountOutput), "\n") {
		cols := strings.Split(line, " ")
		if len(cols) < 3 {
			continue
		}
		if cols[0] == "LazyTree" {
			// We shouldn't have leftover mountpoints, something is wrong.
			return errors.New("Has leftover lazytree mounts")
		}
	}

	return nil
}

func (s *AgentServer) OpenContext(c context.Context, in *proto.OpenContextReq) (*proto.OpenContextResp, error) {
	exposedPorts := nat.PortSet{}

	// Special port for pv_exec
	exposedPorts["50000/tcp"] = struct{}{}
	for _, port := range in.Ports {
		exposedPorts[nat.Port(fmt.Sprintf("%d/tcp", port))] = struct{}{}
	}

	volumes := make(map[string]struct{})
	// Special location for the pv_exec binary
	volumes["/usr/bin/pv_exec"] = struct{}{}

	cmd := strslice.StrSlice{
		"/usr/bin/pv_exec",
		"-host",
		"0.0.0.0",
		"-p",
		"50000",
		"daemon",
		"-account-name",
		in.StorageConfig.AccountName,
		"-account-key",
		in.StorageConfig.AccountKey,
		"-container",
		in.StorageConfig.Container,
		"-source-worktree",
		in.SourceWorktreeId,
		"-worktree",
		in.WorktreeId,
		"-mount-point",
		in.MountPoint,
		"-autosave-interval",
		fmt.Sprintf("%d", in.AutosaveInterval),
	}
	for _, root := range in.ExecConfigRoots {
		cmd = append(cmd, "-exec-config-root", root)
	}

	config := &container.Config{
		Env:          in.Env,
		Image:        in.Image,
		ExposedPorts: exposedPorts,
		Volumes:      volumes,
		Cmd:          cmd,
	}

	binds := []string{
		fmt.Sprintf("%s:/usr/bin/pv_exec:ro", s.ExecBinaryPath),
	}

	portBindings := nat.PortMap{}
	portBindings["50000/tcp"] = []nat.PortBinding{
		nat.PortBinding{
			HostIP: s.HostIp,
		},
	}
	for _, port := range in.Ports {
		portBindings[nat.Port(fmt.Sprintf("%d/tcp", port))] = []nat.PortBinding{
			nat.PortBinding{
				HostIP: s.HostIp,
			},
		}
	}

	// SYS_ADMIN cap is needed to run mount. This is an acceptable
	// security vulnerability I think as the user doesn't have sudo
	// privilege.
	hostConfig := &container.HostConfig{
		CapAdd:       []string{"SYS_ADMIN", "MKNOD"},
		Binds:        binds,
		PortBindings: portBindings,
		SecurityOpt: []string{
			"apparmor:unconfined",
		},
		Resources: container.Resources{
			Devices: []container.DeviceMapping{
				container.DeviceMapping{
					PathOnHost:        "/dev/fuse",
					PathInContainer:   "/dev/fuse",
					CgroupPermissions: "rwm",
				},
			},
		},
	}

	resp, err := s.dockerClient.ContainerCreate(c, config, hostConfig, nil, "")
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Cannot create container, err: "+err.Error())
	}

	containerId := resp.ID
	cleanupContainerFunc := func() {
		s.dockerClient.ContainerStop(c, containerId, nil)
		s.dockerClient.ContainerRemove(c, containerId, types.ContainerRemoveOptions{})
	}

	err = s.dockerClient.ContainerStart(c, containerId, types.ContainerStartOptions{})
	if err != nil {
		cleanupContainerFunc()
		return nil, grpc.Errorf(codes.Internal, "Cannot start container, err: "+err.Error())
	}

	json, err := s.dockerClient.ContainerInspect(c, containerId)
	if err != nil {
		cleanupContainerFunc()
		return nil, grpc.Errorf(codes.Internal, "Cannot inspect container, err: "+err.Error())
	}

	grpcEndpoint := ""
	portEndpoints := []*proto.OpenContextResp_PortEndpoint{}
	for port, bindings := range json.NetworkSettings.Ports {
		if port.Int() == 50000 && bindings != nil && len(bindings) > 0 {
			grpcEndpoint = fmt.Sprintf("%s:%s", bindings[0].HostIP, bindings[0].HostPort)
		} else if bindings != nil && len(bindings) > 0 {
			portEndpoint := &proto.OpenContextResp_PortEndpoint{
				Port:     uint32(port.Int()),
				Endpoint: fmt.Sprintf("%s:%s", bindings[0].HostIP, bindings[0].HostPort),
			}
			portEndpoints = append(portEndpoints, portEndpoint)
		}
	}

	if grpcEndpoint == "" || len(portEndpoints) != len(in.Ports) {
		cleanupContainerFunc()
		return nil, grpc.Errorf(codes.Internal, "Not enough port mapping, ports: %v", json.NetworkSettings.Ports)
	}

	// Wait for grpc server to show up
	// TODO: the default backoff is 1s after the first failure, and cannot
	// be changed. This results in a default 1s delay here. Might need a
	// better synchronization mechanism.
	// Sleep for a short while to mitigate this
	<-time.After(time.Millisecond * 200)
	startTime := time.Now()
	grpcConn, err := grpc.Dial(grpcEndpoint,
		grpc.WithBackoffMaxDelay(time.Millisecond*10),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(time.Second*10))
	if err != nil {
		cleanupContainerFunc()
		return nil, grpc.Errorf(codes.Internal, "GRPC connection is not available: ", err.Error())
	}
	log.Printf("[INFO] Waiting for GRPC server takes %fs", time.Since(startTime).Seconds())
	grpcConn.Close()

	return &proto.OpenContextResp{
		ContextId:     containerId,
		GrpcEndpoint:  grpcEndpoint,
		PortEndpoints: portEndpoints,
	}, nil
}

func (s *AgentServer) CloseContext(c context.Context, in *proto.CloseContextReq) (*proto.CloseContextResp, error) {
	timeout := time.Second * 60
	err := s.dockerClient.ContainerStop(c, in.ContextId, &timeout)
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		log.Println("Timeout during ContainerStop, contextId:", in.ContextId)
		// Try to kill the container
		err = s.dockerClient.ContainerKill(c, in.ContextId, "SIGKILL")
		if err != nil {
			return nil, grpc.Errorf(codes.Internal, "Cannot kill container, err: "+err.Error())
		}
	} else if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Cannot stop container, err: "+err.Error())
	}

	err = s.dockerClient.ContainerRemove(c, in.ContextId, types.ContainerRemoveOptions{})
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Cannot remove container, err: "+err.Error())
	}

	return &proto.CloseContextResp{}, nil
}

func (s *AgentServer) CloseAll(c context.Context, in *proto.CloseAllReq) (*proto.CloseAllResp, error) {
	err := s.Cleanup()
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Failed to close all contexts, err: "+err.Error())
	} else {
		return &proto.CloseAllResp{}, nil
	}
}
