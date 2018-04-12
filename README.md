## pv_agent

`pv_agent` is a per-host daemon that is responsible for starting and
stopping development containers. It is basically a wrapper of Docker APIs.  It
exposes a GRPC service to control the containers.

In Postverta terminology, a container is called a "context" because we want to
make the rest of the system agnostic about how a workspace is implemented (it
can be a container, or a virtual machine, or something else).

## Usage

Build the binary and run it. It can be run as a daemon (with the `daemon`
sub-command) or as a test GRPC client (with the `open` and `close`
sub-command). It requires the docker daemon to be running and will try to
access the well-known UNIX socket at `/var/run/docker.sock`. It also requires a
path to the `pv_exec` binary.
