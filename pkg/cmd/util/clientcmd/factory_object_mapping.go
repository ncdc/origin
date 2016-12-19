package clientcmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/emicklei/go-restful/swagger"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/homedir"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/restmapper"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationreaper "github.com/openshift/origin/pkg/authorization/reaper"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildcmd "github.com/openshift/origin/pkg/build/cmd"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploycmd "github.com/openshift/origin/pkg/deploy/cmd"
	userapi "github.com/openshift/origin/pkg/user/api"
	authenticationreaper "github.com/openshift/origin/pkg/user/reaper"
)

type ring1Factory struct {
	clientAccessFactory     kcmdutil.ClientAccessFactory
	kubeObjetMappingFactory kcmdutil.ObjectMappingFactory
}

func NewObjectMappingFactory(clientAccessFactory kcmdutil.ClientAccessFactory) kcmdutil.ObjectMappingFactory {
	return &ring1Factory{
		clientAccessFactory:     clientAccessFactory,
		kubeObjetMappingFactory: kcmdutil.NewObjectMappingFactory(clientAccessFactory),
	}
}

func (f *ring1Factory) Object() (meta.RESTMapper, runtime.ObjectTyper) {
	defaultMapper := ShortcutExpander{RESTMapper: kcmdutil.ShortcutExpander{RESTMapper: registered.RESTMapper()}}
	defaultTyper := kapi.Scheme

	// Output using whatever version was negotiated in the client cache. The
	// version we decode with may not be the same as what the server requires.
	cfg, err := f.clients.ClientConfigForVersion(nil)
	if err != nil {
		return defaultMapper, defaultTyper
	}

	cmdApiVersion := unversioned.GroupVersion{}
	if cfg.GroupVersion != nil {
		cmdApiVersion = *cfg.GroupVersion
	}

	// at this point we've negotiated and can get the client
	oclient, err := f.clients.ClientForVersion(nil)
	if err != nil {
		return defaultMapper, defaultTyper
	}

	cacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube"), cfg.Host)
	cachedDiscoverClient := NewCachedDiscoveryClient(client.NewDiscoveryClient(oclient.RESTClient), cacheDir, time.Duration(10*time.Minute))

	// if we can't find the server version or its too old to have Kind information in the discovery doc, skip the discovery RESTMapper
	// and use our hardcoded levels
	mapper := registered.RESTMapper()
	if serverVersion, err := cachedDiscoverClient.ServerVersion(); err == nil && useDiscoveryRESTMapper(serverVersion.GitVersion) {
		mapper = restmapper.NewDiscoveryRESTMapper(cachedDiscoverClient)
	}
	mapper = NewShortcutExpander(cachedDiscoverClient, kcmdutil.ShortcutExpander{RESTMapper: mapper})
	return kubectl.OutputVersionMapper{RESTMapper: mapper, OutputVersions: []unversioned.GroupVersion{cmdApiVersion}}, kapi.Scheme
}

func (f *ring1Factory) UnstructuredObject() (meta.RESTMapper, runtime.ObjectTyper, error) {
	// load a discovery client from the default config
	cfg, err := f.clients.ClientConfigForVersion(nil)
	if err != nil {
		return nil, nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	cacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube"), cfg.Host)
	cachedDiscoverClient := NewCachedDiscoveryClient(client.NewDiscoveryClient(dc.RESTClient()), cacheDir, time.Duration(10*time.Minute))

	// enumerate all group resources
	groupResources, err := discovery.GetAPIGroupResources(cachedDiscoverClient)
	if err != nil {
		return nil, nil, err
	}

	// Register unknown APIs as third party for now to make
	// validation happy. TODO perhaps make a dynamic schema
	// validator to avoid this.
	for _, group := range groupResources {
		for _, version := range group.Group.Versions {
			gv := unversioned.GroupVersion{Group: group.Group.Name, Version: version.Version}
			if !registered.IsRegisteredVersion(gv) {
				registered.AddThirdPartyAPIGroupVersions(gv)
			}
		}
	}

	// construct unstructured mapper and typer
	mapper := discovery.NewRESTMapper(groupResources, meta.InterfacesForUnstructured)
	typer := discovery.NewUnstructuredObjectTyper(groupResources)
	return NewShortcutExpander(cachedDiscoverClient, kcmdutil.ShortcutExpander{RESTMapper: mapper}), typer, nil
}

func (f *ring1Factory) ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	if latest.OriginKind(mapping.GroupVersionKind) {
		mappingVersion := mapping.GroupVersionKind.GroupVersion()
		client, err := f.clients.ClientForVersion(&mappingVersion)
		if err != nil {
			return nil, err
		}
		return client.RESTClient, nil
	}
	return f.kubeObjetMappingFactory.ClientForMapping(mapping)
}

func (f *ring1Factory) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	if latest.OriginKind(mapping.GroupVersionKind) {
		cfg, err := f.OpenShiftClientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
		if err := client.SetOpenShiftDefaults(cfg); err != nil {
			return nil, err
		}
		cfg.APIPath = "/apis"
		if mapping.GroupVersionKind.Group == kapi.GroupName {
			cfg.APIPath = "/oapi"
		}
		gv := mapping.GroupVersionKind.GroupVersion()
		cfg.ContentConfig = dynamic.ContentConfig()
		cfg.GroupVersion = &gv
		return restclient.RESTClientFor(cfg)
	}
	return f.kubeObjetMappingFactory.UnstructuredClientForMapping(mapping)
}

func (f *ring1Factory) Describer(mapping *meta.RESTMapping) (kubectl.Describer, error) {
	if latest.OriginKind(mapping.GroupVersionKind) {
		oClient, kClient, err := f.Clients()
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.GroupVersionKind.Kind, err)
		}

		mappingVersion := mapping.GroupVersionKind.GroupVersion()
		cfg, err := f.clients.ClientConfigForVersion(&mappingVersion)
		if err != nil {
			return nil, fmt.Errorf("unable to load a client %s: %v", mapping.GroupVersionKind.Kind, err)
		}

		describer, ok := describe.DescriberFor(mapping.GroupVersionKind.GroupKind(), oClient, kClient, cfg.Host)
		if !ok {
			return nil, fmt.Errorf("no description has been implemented for %q", mapping.GroupVersionKind.Kind)
		}
		return describer, nil
	}
	return f.kubeObjetMappingFactory.Describer(mapping)
}

func (f *ring1Factory) LogsForObject(object, options runtime.Object) (*restclient.Request, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		dopts, ok := options.(*deployapi.DeploymentLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a DeploymentLogOptions")
		}
		oc, _, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return oc.DeploymentLogs(t.Namespace).Get(t.Name, *dopts), nil
	case *buildapi.Build:
		bopts, ok := options.(*buildapi.BuildLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a BuildLogOptions")
		}
		if bopts.Version != nil {
			return nil, errors.New("cannot specify a version and a build")
		}
		oc, _, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return oc.BuildLogs(t.Namespace).Get(t.Name, *bopts), nil
	case *buildapi.BuildConfig:
		bopts, ok := options.(*buildapi.BuildLogOptions)
		if !ok {
			return nil, errors.New("provided options object is not a BuildLogOptions")
		}
		oc, _, err := f.Clients()
		if err != nil {
			return nil, err
		}
		builds, err := oc.Builds(t.Namespace).List(kapi.ListOptions{})
		if err != nil {
			return nil, err
		}
		builds.Items = buildapi.FilterBuilds(builds.Items, buildapi.ByBuildConfigPredicate(t.Name))
		if len(builds.Items) == 0 {
			return nil, fmt.Errorf("no builds found for %q", t.Name)
		}
		if bopts.Version != nil {
			// If a version has been specified, try to get the logs from that build.
			desired := buildutil.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
			return oc.BuildLogs(t.Namespace).Get(desired, *bopts), nil
		}
		sort.Sort(sort.Reverse(buildapi.BuildSliceByCreationTimestamp(builds.Items)))
		return oc.BuildLogs(t.Namespace).Get(builds.Items[0].Name, *bopts), nil
	default:
		return f.kubeObjetMappingFactory.LogsForObject(object, options)
	}
}

func (f *ring1Factory) Scaler(mapping *meta.RESTMapping) (kubectl.Scaler, error) {
	if mapping.GroupVersionKind.GroupKind() == deployapi.Kind("DeploymentConfig") {
		oc, kc, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigScaler(oc, kc), nil
	}
	return f.kubeObjetMappingFactory.Scaler(mapping)
}

func (f *ring1Factory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	switch mapping.GroupVersionKind.GroupKind() {
	case deployapi.Kind("DeploymentConfig"):
		oc, kc, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigReaper(oc, kc), nil
	case authorizationapi.Kind("Role"):
		oc, _, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return authorizationreaper.NewRoleReaper(oc, oc), nil
	case authorizationapi.Kind("ClusterRole"):
		oc, _, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return authorizationreaper.NewClusterRoleReaper(oc, oc, oc), nil
	case userapi.Kind("User"):
		oc, kc, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return authenticationreaper.NewUserReaper(
			client.UsersInterface(oc),
			client.GroupsInterface(oc),
			client.ClusterRoleBindingsInterface(oc),
			client.RoleBindingsNamespacer(oc),
			client.OAuthClientAuthorizationsInterface(oc),
			kc.Core(),
		), nil
	case userapi.Kind("Group"):
		oc, kc, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return authenticationreaper.NewGroupReaper(
			client.GroupsInterface(oc),
			client.ClusterRoleBindingsInterface(oc),
			client.RoleBindingsNamespacer(oc),
			kc.Core(),
		), nil
	case buildapi.Kind("BuildConfig"):
		oc, _, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return buildcmd.NewBuildConfigReaper(oc), nil
	}
	return f.kubeObjetMappingFactory.Reaper(mapping)
}

func (f *ring1Factory) HistoryViewer(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
	switch mapping.GroupVersionKind.GroupKind() {
	case deployapi.Kind("DeploymentConfig"):
		oc, kc, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigHistoryViewer(oc, kc), nil
	}
	return f.kubeObjetMappingFactory.HistoryViewer(mapping)
}

func (f *ring1Factory) Rollbacker(mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
	switch mapping.GroupVersionKind.GroupKind() {
	case deployapi.Kind("DeploymentConfig"):
		oc, _, err := f.Clients()
		if err != nil {
			return nil, err
		}
		return deploycmd.NewDeploymentConfigRollbacker(oc), nil
	}
	return f.kubeObjetMappingFactory.Rollbacker(mapping)
}

func (f *ring1Factory) StatusViewer(mapping *meta.RESTMapping) (kubectl.StatusViewer, error) {
	oc, _, err := f.Clients()
	if err != nil {
		return nil, err
	}

	switch mapping.GroupVersionKind.GroupKind() {
	case deployapi.Kind("DeploymentConfig"):
		return deploycmd.NewDeploymentConfigStatusViewer(oc), nil
	}
	return f.kubeObjetMappingFactory.StatusViewer(mapping)
}

func (f *ring1Factory) AttachablePodForObject(object runtime.Object) (*kapi.Pod, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		_, kc, err := f.Clients()
		if err != nil {
			return nil, err
		}
		selector := labels.SelectorFromSet(t.Spec.Selector)
		f := func(pods []*kapi.Pod) sort.Interface { return sort.Reverse(controller.ActivePods(pods)) }
		pod, _, err := kcmdutil.GetFirstPod(kc, t.Namespace, selector, 1*time.Minute, f)
		return pod, err
	default:
		return f.kubeObjetMappingFactory.AttachablePodForObject(object)
	}
}

// PrinterForMapping returns a printer suitable for displaying the provided resource type.
// Requires that printer flags have been added to cmd (see AddPrinterFlags).
func (f *ring1Factory) PrinterForMapping(cmd *cobra.Command, mapping *meta.RESTMapping, withNamespace bool) (kubectl.ResourcePrinter, error) {
	// TODO FIX ME. COPIED FROM KUBE AS PART OF THE COPY/PASTE FOR
	// PrinterForMapping
	if latest.OriginKind(mapping.GroupVersionKind) {
		printer, ok, err := kcmdutil.PrinterForCommand(cmd)
		if err != nil {
			return nil, err
		}
		if ok {
			clientConfig, err := f.ClientConfig()
			if err != nil {
				return nil, err
			}

			version, err := kcmdutil.OutputVersion(cmd, clientConfig.GroupVersion)
			if err != nil {
				return nil, err
			}
			if version.Empty() && mapping != nil {
				version = mapping.GroupVersionKind.GroupVersion()
			}
			if version.Empty() {
				return nil, fmt.Errorf("you must specify an output-version when using this output format")
			}

			if mapping != nil {
				printer = kubectl.NewVersionedPrinter(printer, mapping.ObjectConvertor, version, mapping.GroupVersionKind.GroupVersion())
			}

		} else {
			// Some callers do not have "label-columns" so we can't use the GetFlagStringSlice() helper
			columnLabel, err := cmd.Flags().GetStringSlice("label-columns")
			if err != nil {
				columnLabel = []string{}
			}
			printer, err = f.Printer(mapping, kubectl.PrintOptions{
				NoHeaders:          kcmdutil.GetFlagBool(cmd, "no-headers"),
				WithNamespace:      withNamespace,
				Wide:               kcmdutil.GetWideFlag(cmd),
				ShowAll:            kcmdutil.GetFlagBool(cmd, "show-all"),
				ShowLabels:         kcmdutil.GetFlagBool(cmd, "show-labels"),
				AbsoluteTimestamps: isWatch(cmd),
				ColumnLabels:       columnLabel,
			})
			if err != nil {
				return nil, err
			}
			printer = maybeWrapSortingPrinter(cmd, printer)
		}

		return printer, nil
	}

	return f.kubeObjetMappingFactory.PrinterForMapping(cmd, mapping, withNamespace)
}

func (f *ring1Factory) Validator(validate bool, cacheDir string) (validation.Schema, error) {
	return f.kubeObjetMappingFactory.Validator(validate, cacheDir)
}

func (f *ring1Factory) SwaggerSchema(gvk unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error) {
	if !latest.OriginKind(gvk) {
		return f.kubeObjetMappingFactory.SwaggerSchema(gvk)
	}
	// TODO: we need to register the OpenShift API under the Kube group, and start returning the OpenShift
	// group from the scheme.
	oc, _, err := f.Clients()
	if err != nil {
		return nil, err
	}
	return f.OriginSwaggerSchema(oc.RESTClient, gvk.GroupVersion())
}

// TODO REMOVE ME. COPIED FROM KUBE AS PART OF THE COPY/PASTE FOR
// PrinterForMapping
func maybeWrapSortingPrinter(cmd *cobra.Command, printer kubectl.ResourcePrinter) kubectl.ResourcePrinter {
	sorting, err := cmd.Flags().GetString("sort-by")
	if err != nil {
		// error can happen on missing flag or bad flag type.  In either case, this command didn't intent to sort
		return printer
	}

	if len(sorting) != 0 {
		return &kubectl.SortingPrinter{
			Delegate:  printer,
			SortField: fmt.Sprintf("{%s}", sorting),
		}
	}
	return printer
}

// useDiscoveryRESTMapper checks the server version to see if its recent enough to have
// enough discovery information avaiable to reliably build a RESTMapper.  If not, use the
// hardcoded mapper in this client (legacy behavior)
func useDiscoveryRESTMapper(serverVersion string) bool {
	serverSemVer, err := semver.Parse(serverVersion[1:])
	if err != nil {
		return false
	}
	if serverSemVer.LT(semver.MustParse("1.3.0")) {
		return false
	}
	return true
}
