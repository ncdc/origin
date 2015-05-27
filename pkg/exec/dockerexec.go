package exec

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"syscall"

	kubecontainer "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/container"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/dockertools"
	"github.com/fsouza/go-dockerclient"
)

// OpenShiftExecHandler uses dockerexec to execute commands in containers
// without going through the Docker daemon. See
// https://github.com/openshift/dockerexec for more details.
// OpenShiftExecHandler implements dockertools.DockerExecHandler.
type OpenShiftExecHandler struct{}

func (*OpenShiftExecHandler) ExecInContainer(client dockertools.DockerInterface, container *docker.Container, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool) error {
	dockerexec, err := exec.LookPath("dockerexec")
	if err != nil {
		return fmt.Errorf("exec unavailable - unable to locate dockerexec")
	}

	args := []string{"exec", "--id", container.ID}
	if tty {
		args = append(args, "-t")
	}
	args = append(args, "--env", fmt.Sprintf("HOSTNAME=%s", container.Config.Hostname))
	args = append(args, cmd...)
	command := exec.Command(dockerexec, args...)

	addr, err := net.ResolveUnixAddr("unix", "/var/run/docker.sock")
	if err != nil {
		return err
	}

	conn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return err
	}

	file, err := conn.File()
	if err != nil {
		return err
	}

	ucred, err := syscall.GetsockoptUcred(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		return err
	}

	command.Env = []string{fmt.Sprintf("_DOCKER_PID=%d", ucred.Pid)}

	if tty {
		p, err := kubecontainer.StartPty(command)
		if err != nil {
			return err
		}
		defer p.Close()

		// make sure to close the stdout stream
		defer stdout.Close()

		if stdin != nil {
			go io.Copy(p, stdin)
		}

		if stdout != nil {
			go io.Copy(stdout, p)
		}

		return command.Wait()
	} else {
		if stdin != nil {
			// Use an os.Pipe here as it returns true *os.File objects.
			// This way, if you run 'kubectl exec -p <pod> -i bash' (no tty) and type 'exit',
			// the call below to command.Run() can unblock because its Stdin is the read half
			// of the pipe.
			r, w, err := os.Pipe()
			if err != nil {
				return err
			}
			go io.Copy(w, stdin)

			command.Stdin = r
		}
		if stdout != nil {
			command.Stdout = stdout
		}
		if stderr != nil {
			command.Stderr = stderr
		}

		return command.Run()
	}
}
