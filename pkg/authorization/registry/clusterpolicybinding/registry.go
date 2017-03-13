package clusterpolicybinding

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

// Registry is an interface for things that know how to store ClusterPolicyBindings.
type Registry interface {
	// ListClusterPolicyBindings obtains list of policyBindings that match a selector.
	ListClusterPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error)
	// GetClusterPolicyBinding retrieves a specific policyBinding.
	GetClusterPolicyBinding(ctx apirequest.Context, name string) (*authorizationapi.ClusterPolicyBinding, error)
	// CreateClusterPolicyBinding creates a new policyBinding.
	CreateClusterPolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error
	// UpdateClusterPolicyBinding updates a policyBinding.
	UpdateClusterPolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error
	// DeleteClusterPolicyBinding deletes a policyBinding.
	DeleteClusterPolicyBinding(ctx apirequest.Context, name string) error
}

type WatchingRegistry interface {
	Registry
	// WatchClusterPolicyBindings watches policyBindings.
	WatchClusterPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
}

type ReadOnlyClusterPolicyInterface interface {
	List(options metainternal.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error)
	Get(name string) (*authorizationapi.ClusterPolicyBinding, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.StandardStorage
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage) WatchingRegistry {
	return &storage{s}
}

func (s *storage) ListClusterPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}

	return obj.(*authorizationapi.ClusterPolicyBindingList), nil
}

func (s *storage) CreateClusterPolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	_, err := s.Create(ctx, policyBinding)
	return err
}

func (s *storage) UpdateClusterPolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	_, _, err := s.Update(ctx, policyBinding.Name, rest.DefaultUpdatedObjectInfo(policyBinding, kapi.Scheme))
	return err
}

func (s *storage) WatchClusterPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetClusterPolicyBinding(ctx apirequest.Context, name string) (*authorizationapi.ClusterPolicyBinding, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*authorizationapi.ClusterPolicyBinding), nil
}

func (s *storage) DeleteClusterPolicyBinding(ctx apirequest.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	return err
}

type simulatedStorage struct {
	clusterRegistry Registry
}

func NewSimulatedRegistry(clusterRegistry Registry) policybinding.Registry {
	return &simulatedStorage{clusterRegistry}
}

func (s *simulatedStorage) ListPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	ret, err := s.clusterRegistry.ListClusterPolicyBindings(ctx, options)
	return authorizationapi.ToPolicyBindingList(ret), err
}

func (s *simulatedStorage) CreatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error {
	return s.clusterRegistry.CreateClusterPolicyBinding(ctx, authorizationapi.ToClusterPolicyBinding(policyBinding))
}

func (s *simulatedStorage) UpdatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error {
	return s.clusterRegistry.UpdateClusterPolicyBinding(ctx, authorizationapi.ToClusterPolicyBinding(policyBinding))
}

func (s *simulatedStorage) GetPolicyBinding(ctx apirequest.Context, name string) (*authorizationapi.PolicyBinding, error) {
	ret, err := s.clusterRegistry.GetClusterPolicyBinding(ctx, name)
	return authorizationapi.ToPolicyBinding(ret), err
}

func (s *simulatedStorage) DeletePolicyBinding(ctx apirequest.Context, name string) error {
	return s.clusterRegistry.DeleteClusterPolicyBinding(ctx, name)
}

type ReadOnlyClusterPolicyBinding struct {
	Registry
}

func (s ReadOnlyClusterPolicyBinding) List(options metainternal.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	return s.ListClusterPolicyBindings(apirequest.WithNamespace(apirequest.NewContext(), ""), &options)
}

func (s ReadOnlyClusterPolicyBinding) Get(name string) (*authorizationapi.ClusterPolicyBinding, error) {
	return s.GetClusterPolicyBinding(apirequest.WithNamespace(apirequest.NewContext(), ""), name)
}
