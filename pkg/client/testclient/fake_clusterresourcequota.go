package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/watch"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotaInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterResourceQuotas struct {
	Fake *Fake
}

var clusteResourceQuotasResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterresourcequotas"}

func (c *FakeClusterResourceQuotas) Get(name string) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(clusteResourceQuotasResource, name), &quotaapi.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts metainternal.ListOptions) (*quotaapi.ClusterResourceQuotaList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(clusteResourceQuotasResource, opts), &quotaapi.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuotaList), err
}

func (c *FakeClusterResourceQuotas) Create(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(clusteResourceQuotasResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(clusteResourceQuotasResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}
func (c *FakeClusterResourceQuotas) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(clusteResourceQuotasResource, name), &quotaapi.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewRootWatchAction(clusteResourceQuotasResource, opts))
}

func (c *FakeClusterResourceQuotas) UpdateStatus(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	action := core.UpdateActionImpl{}
	action.Verb = "update"
	action.Resource = clusteResourceQuotasResource
	action.Subresource = "status"
	action.Object = inObj

	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err

}
