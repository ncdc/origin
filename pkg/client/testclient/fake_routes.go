package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// FakeRoutes implements RouteInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeRoutes struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeRoutes) Get(name string) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(routeapi.SchemeGroupVersion.WithResource("routes"), c.Namespace, name), &routeapi.Route{})
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) List(opts kapi.ListOptions) (*routeapi.RouteList, error) {
	obj, err := c.Fake.Invokes(core.NewListAction(routeapi.SchemeGroupVersion.WithResource("routes"), c.Namespace, opts), &routeapi.RouteList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.RouteList), err
}

func (c *FakeRoutes) Create(inObj *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(routeapi.SchemeGroupVersion.WithResource("routes"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Update(inObj *routeapi.Route) (*routeapi.Route, error) {
	obj, err := c.Fake.Invokes(core.NewUpdateAction(routeapi.SchemeGroupVersion.WithResource("routes"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) UpdateStatus(inObj *routeapi.Route) (*routeapi.Route, error) {
	action := core.NewUpdateAction(routeapi.SchemeGroupVersion.WithResource("routes"), c.Namespace, inObj)
	action.Subresource = "status"
	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*routeapi.Route), err
}

func (c *FakeRoutes) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewDeleteAction(routeapi.SchemeGroupVersion.WithResource("routes"), c.Namespace, name), &routeapi.Route{})
	return err
}

func (c *FakeRoutes) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewWatchAction(routeapi.SchemeGroupVersion.WithResource("routes"), c.Namespace, opts))
}
