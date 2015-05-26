package app

// This file exists to force the desired plugin implementations to be linked.
import (
	// Credential providers
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"syscall"

	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/credentialprovider/gcp"
	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	// Network plugins
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/dockertools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/network"
	kexec "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/network/exec"
	// Volume plugins
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/empty_dir"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/gce_pd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/git_repo"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/host_path"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/nfs"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/volume/secret"

	kubecontainer "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/container"
)

// ProbeVolumePlugins collects all volume plugins into an easy to use list.
func ProbeVolumePlugins() []volume.VolumePlugin {
	allPlugins := []volume.VolumePlugin{}

	// The list of plugins to probe is decided by the kubelet binary, not
	// by dynamic linking or other "magic".  Plugins will be analyzed and
	// initialized later.
	allPlugins = append(allPlugins, empty_dir.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, gce_pd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, git_repo.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, host_path.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, secret.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins()...)

	return allPlugins
}

// ProbeNetworkPlugins collects all compiled-in plugins
func ProbeNetworkPlugins() []network.NetworkPlugin {
	allPlugins := []network.NetworkPlugin{}

	// for each existing plugin, add to the list
	allPlugins = append(allPlugins, kexec.ProbeNetworkPlugins()...)

	return allPlugins
}

func init() {
	dockertools.RegisterExecHandler("daemonless-dockerexec", &DaemonlessExecHandler{})
}

type DaemonlessExecHandler struct {
}

func (deh *DaemonlessExecHandler) ExecInContainer(container *docker.Container, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, winch io.Reader) error {
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

		if winch != nil {
			go func() {
				var w, h uint16
				for {
					err := binary.Read(winch, binary.LittleEndian, &w)
					if err != nil {
						glog.Errorf("Error reading width: %v", err)
						return
					}

					err = binary.Read(winch, binary.LittleEndian, &h)
					if err != nil {
						glog.Errorf("Error reading height: %v", err)
						return
					}

					err = term.SetWinsize(p.Fd(), &term.Winsize{Height: h, Width: w})
					if err != nil {
						glog.Errorf("Error setting winsize: %v", err)
					}
				}
			}()
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
