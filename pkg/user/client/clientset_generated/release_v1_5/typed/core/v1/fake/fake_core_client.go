package fake

import (
	v1 "github.com/openshift/origin/pkg/user/client/clientset_generated/release_v1_5/typed/core/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeCoreV1 struct {
	*testing.Fake
}

func (c *FakeCoreV1) Users(namespace string) v1.UserInterface {
	return &FakeUsers{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeCoreV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
