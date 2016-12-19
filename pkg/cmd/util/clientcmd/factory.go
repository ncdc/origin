package clientcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/emicklei/go-restful/swagger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/controller"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// New creates a default Factory for commands that should share identical server
// connection behavior. Most commands should use this method to get a factory.
func New(flags *pflag.FlagSet) kcmdutil.Factory {
	f := NewFactory(nil)
	f.BindFlags(flags)

	return f
}

// Factory provides common options for OpenShift commands
type factory struct {
	kcmdutil.ClientAccessFactory
	kcmdutil.ObjectMappingFactory
	kcmdutil.BuilderFactory
}

var _ kcmdutil.Factory = &factory{}

// NewFactory creates an object that holds common methods across all OpenShift commands
func NewFactory(optionalClientConfig kclientcmd.ClientConfig) kcmdutil.Factory {
	// clients := &clientCache{
	// 	clients: make(map[string]*client.Client),
	// 	configs: make(map[string]*restclient.Config),
	// 	loader:  clientConfig,
	// }

	// w := &Factory{
	// 	Factory:                kcmdutil.NewFactory(clientConfig),
	// 	OpenShiftClientConfig:  clientConfig,
	// 	clients:                clients,
	// 	ImageResolutionOptions: &imageResolutionOptions{},
	// }

	// return w

	clientAccessFactory := NewClientAccessFactory(optionalClientConfig)
	objectMappingFactory := NewObjectMappingFactory(clientAccessFactory)
	builderFactory := kcmdutil.NewBuilderFactory(clientAccessFactory, objectMappingFactory)

	return &factory{
		ClientAccessFactory:  clientAccessFactory,
		ObjectMappingFactory: objectMappingFactory,
		BuilderFactory:       builderFactory,
	}
}

// TODO REMOVE ME. COPIED FROM KUBE AS PART OF THE COPY/PASTE FOR
// PrinterForMapping
// Whether this cmd need watching objects.
func isWatch(cmd *cobra.Command) bool {
	if w, err := cmd.Flags().GetBool("watch"); w && err == nil {
		return true
	}

	if wo, err := cmd.Flags().GetBool("watch-only"); wo && err == nil {
		return true
	}

	return false
}

// PrintResourceInfos receives a list of resource infos and prints versioned objects if a generic output format was specified
// otherwise, it iterates through info objects, printing each resource with a unique printer for its mapping
func (f *factory) PrintResourceInfos(cmd *cobra.Command, infos []*resource.Info, out io.Writer) error {
	printer, generic, err := kcmdutil.PrinterForCommand(cmd)
	if err != nil {
		return nil
	}
	if !generic {
		for _, info := range infos {
			mapping := info.ResourceMapping()
			printer, err := f.PrinterForMapping(cmd, mapping, false)
			if err != nil {
				return err
			}
			if err := printer.PrintObj(info.Object, out); err != nil {
				return nil
			}
		}
		return nil
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	outputVersion, err := kcmdutil.OutputVersion(cmd, clientConfig.GroupVersion)
	if err != nil {
		return err
	}
	object, err := resource.AsVersionedObject(infos, len(infos) != 1, outputVersion, api.Codecs.LegacyCodec(outputVersion))
	if err != nil {
		return err
	}
	return printer.PrintObj(object, out)

}

// FlagBinder represents an interface that allows to bind extra flags into commands.
type FlagBinder interface {
	// Bound returns true if the flag is already bound to a command.
	Bound() bool
	// Bind allows to bind an extra flag to a command
	Bind(*pflag.FlagSet)
}

func ResourceMapper(f kcmdutil.Factory) *resource.Mapper {
	mapper, typer := f.Object()
	return &resource.Mapper{
		RESTMapper:   mapper,
		ObjectTyper:  typer,
		ClientMapper: resource.ClientMapperFunc(f.ClientForMapping),
	}
}

// UpdateObjectEnvironment update the environment variables in object specification.
func (f *factory) UpdateObjectEnvironment(obj runtime.Object, fn func(*[]api.EnvVar) error) (bool, error) {
	switch t := obj.(type) {
	case *buildapi.BuildConfig:
		if t.Spec.Strategy.CustomStrategy != nil {
			return true, fn(&t.Spec.Strategy.CustomStrategy.Env)
		}
		if t.Spec.Strategy.SourceStrategy != nil {
			return true, fn(&t.Spec.Strategy.SourceStrategy.Env)
		}
		if t.Spec.Strategy.DockerStrategy != nil {
			return true, fn(&t.Spec.Strategy.DockerStrategy.Env)
		}
	}
	return false, fmt.Errorf("object does not contain any environment variables")
}

// ExtractFileContents returns a map of keys to contents, false if the object cannot support such an
// operation, or an error.
func (f *factory) ExtractFileContents(obj runtime.Object) (map[string][]byte, bool, error) {
	switch t := obj.(type) {
	case *api.Secret:
		return t.Data, true, nil
	case *api.ConfigMap:
		out := make(map[string][]byte)
		for k, v := range t.Data {
			out[k] = []byte(v)
		}
		return out, true, nil
	default:
		return nil, false, nil
	}
}

// ApproximatePodTemplateForObject returns a pod template object for the provided source.
// It may return both an error and a object. It attempt to return the best possible template
// available at the current time.
func (f *factory) ApproximatePodTemplateForObject(object runtime.Object) (*api.PodTemplateSpec, error) {
	switch t := object.(type) {
	case *imageapi.ImageStreamTag:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &api.PodTemplateSpec{
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyNever,
				Containers: []api.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *imageapi.ImageStreamImage:
		// create a minimal pod spec that uses the image referenced by the istag without any introspection
		// it possible that we could someday do a better job introspecting it
		return &api.PodTemplateSpec{
			Spec: api.PodSpec{
				RestartPolicy: api.RestartPolicyNever,
				Containers: []api.Container{
					{Name: "container-00", Image: t.Image.DockerImageReference},
				},
			},
		}, nil
	case *deployapi.DeploymentConfig:
		fallback := t.Spec.Template

		_, kc, err := f.Clients()
		if err != nil {
			return fallback, err
		}

		latestDeploymentName := deployutil.LatestDeploymentNameForConfig(t)
		deployment, err := kc.ReplicationControllers(t.Namespace).Get(latestDeploymentName)
		if err != nil {
			return fallback, err
		}

		fallback = deployment.Spec.Template

		pods, err := kc.Pods(deployment.Namespace).List(api.ListOptions{LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector)})
		if err != nil {
			return fallback, err
		}

		for i := range pods.Items {
			pod := &pods.Items[i]
			if fallback == nil || pod.CreationTimestamp.Before(fallback.CreationTimestamp) {
				fallback = &api.PodTemplateSpec{
					ObjectMeta: pod.ObjectMeta,
					Spec:       pod.Spec,
				}
			}
		}
		return fallback, nil

	default:
		pod, err := f.AttachablePodForObject(object)
		if pod != nil {
			return &api.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}, err
		}
		switch t := object.(type) {
		case *api.ReplicationController:
			return t.Spec.Template, err
		case *extensions.ReplicaSet:
			return &t.Spec.Template, err
		case *extensions.DaemonSet:
			return &t.Spec.Template, err
		case *batch.Job:
			return &t.Spec.Template, err
		}
		return nil, err
	}
}

func (f *factory) PodForResource(resource string, timeout time.Duration) (string, error) {
	sortBy := func(pods []*api.Pod) sort.Interface { return sort.Reverse(controller.ActivePods(pods)) }
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return "", err
	}
	mapper, _ := f.Object()
	resourceType, name, err := util.ResolveResource(api.Resource("pods"), resource, mapper)
	if err != nil {
		return "", err
	}

	switch resourceType {
	case api.Resource("pods"):
		return name, nil
	case api.Resource("replicationcontrollers"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		rc, err := kc.Core().ReplicationControllers(namespace).Get(name)
		if err != nil {
			return "", err
		}
		selector := labels.SelectorFromSet(rc.Spec.Selector)
		pod, _, err := kcmdutil.GetFirstPod(kc, namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case deployapi.Resource("deploymentconfigs"):
		oc, kc, err := f.Clients()
		if err != nil {
			return "", err
		}
		dc, err := oc.DeploymentConfigs(namespace).Get(name)
		if err != nil {
			return "", err
		}
		selector := labels.SelectorFromSet(dc.Spec.Selector)
		pod, _, err := kcmdutil.GetFirstPod(kc, namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case extensions.Resource("daemonsets"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		ds, err := kc.Extensions().DaemonSets(namespace).Get(name)
		if err != nil {
			return "", err
		}
		selector, err := unversioned.LabelSelectorAsSelector(ds.Spec.Selector)
		if err != nil {
			return "", err
		}
		pod, _, err := kcmdutil.GetFirstPod(kc, namespace, selector, timeout, sortBy)
		if err != nil {
			return "", err
		}
		return pod.Name, nil
	case extensions.Resource("jobs"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		// TODO/REBASE kc.Extensions() doesn't exist any more. Is this ok?
		job, err := kc.Batch().Jobs(namespace).Get(name)
		if err != nil {
			return "", err
		}
		return podNameForJob(job, kc, timeout, sortBy)
	case batch.Resource("jobs"):
		kc, err := f.ClientSet()
		if err != nil {
			return "", err
		}
		job, err := kc.Batch().Jobs(namespace).Get(name)
		if err != nil {
			return "", err
		}
		return podNameForJob(job, kc, timeout, sortBy)
	default:
		return "", fmt.Errorf("remote shell for %s is not supported", resourceType)
	}
}

func podNameForJob(job *batch.Job, kc kclientset.Interface, timeout time.Duration, sortBy func(pods []*api.Pod) sort.Interface) (string, error) {
	selector, err := unversioned.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return "", err
	}
	pod, _, err := kcmdutil.GetFirstPod(kc.Core(), job.Namespace, selector, timeout, sortBy)
	if err != nil {
		return "", err
	}
	return pod.Name, nil
}

// Clients returns an OpenShift and Kubernetes client.
func (f *factory) Clients() (*client.Client, *kclientset.Clientset, error) {
	kClientset, err := f.ClientSet()
	if err != nil {
		return nil, nil, err
	}
	// TODO: how to get origin client?
	osClient, err := f.clients.ClientForVersion(nil)
	if err != nil {
		return nil, nil, err
	}
	return osClient, kClientset, nil
}

// OriginSwaggerSchema returns a swagger API doc for an Origin schema under the /oapi prefix.
func (f *factory) OriginSwaggerSchema(client *restclient.RESTClient, version unversioned.GroupVersion) (*swagger.ApiDeclaration, error) {
	if version.Empty() {
		return nil, fmt.Errorf("groupVersion cannot be empty")
	}
	body, err := client.Get().AbsPath("/").Suffix("swaggerapi", "oapi", version.Version).Do().Raw()
	if err != nil {
		return nil, err
	}
	var schema swagger.ApiDeclaration
	err = json.Unmarshal(body, &schema)
	if err != nil {
		return nil, fmt.Errorf("got '%s': %v", string(body), err)
	}
	return &schema, nil
}

// FindAllCanonicalResources returns all resource names that map directly to their kind (Kind -> Resource -> Kind)
// and are not subresources. This is the closest mapping possible from the client side to resources that can be
// listed and updated. Note that this may return some virtual resources (like imagestreamtags) that can be otherwise
// represented.
// TODO: add a field to APIResources for "virtual" (or that points to the canonical resource).
// TODO: fallback to the scheme when discovery is not possible.
func FindAllCanonicalResources(d discovery.DiscoveryInterface, m meta.RESTMapper) ([]unversioned.GroupResource, error) {
	set := make(map[unversioned.GroupResource]struct{})
	all, err := d.ServerResources()
	if err != nil {
		return nil, err
	}
	for apiVersion, v := range all {
		gv, err := unversioned.ParseGroupVersion(apiVersion)
		if err != nil {
			continue
		}
		for _, r := range v.APIResources {
			// ignore subresources
			if strings.Contains(r.Name, "/") {
				continue
			}
			// because discovery info doesn't tell us whether the object is virtual or not, perform a lookup
			// by the kind for resource (which should be the canonical resource) and then verify that the reverse
			// lookup (KindsFor) does not error.
			if mapping, err := m.RESTMapping(unversioned.GroupKind{Group: gv.Group, Kind: r.Kind}, gv.Version); err == nil {
				if _, err := m.KindsFor(mapping.GroupVersionKind.GroupVersion().WithResource(mapping.Resource)); err == nil {
					set[unversioned.GroupResource{Group: mapping.GroupVersionKind.Group, Resource: mapping.Resource}] = struct{}{}
				}
			}
		}
	}
	var groupResources []unversioned.GroupResource
	for k := range set {
		groupResources = append(groupResources, k)
	}
	sort.Sort(groupResourcesByName(groupResources))
	return groupResources, nil
}

type groupResourcesByName []unversioned.GroupResource

func (g groupResourcesByName) Len() int { return len(g) }
func (g groupResourcesByName) Less(i, j int) bool {
	if g[i].Resource < g[j].Resource {
		return true
	}
	if g[i].Resource > g[j].Resource {
		return false
	}
	return g[i].Group < g[j].Group
}
func (g groupResourcesByName) Swap(i, j int) { g[i], g[j] = g[j], g[i] }
