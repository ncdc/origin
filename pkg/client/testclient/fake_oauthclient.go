package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type FakeOAuthClient struct {
	Fake *Fake
}

var oAuthClientsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "oauthclients"}

func (c *FakeOAuthClient) Get(name string) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(core.NewRootGetAction(oAuthClientsResource, name), &oauthapi.OAuthClient{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}

func (c *FakeOAuthClient) List(opts metainternal.ListOptions) (*oauthapi.OAuthClientList, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(oAuthClientsResource, opts), &oauthapi.OAuthClientList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClientList), err
}

func (c *FakeOAuthClient) Create(inObj *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(oAuthClientsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}

func (c *FakeOAuthClient) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(oAuthClientsResource, name), &oauthapi.OAuthClient{})
	return err
}

func (c *FakeOAuthClient) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewRootWatchAction(oAuthClientsResource, opts))
}

func (c *FakeOAuthClient) Update(client *oauthapi.OAuthClient) (*oauthapi.OAuthClient, error) {
	obj, err := c.Fake.Invokes(core.NewRootUpdateAction(oAuthClientsResource, client), &oauthapi.OAuthClient{})
	if obj == nil {
		return nil, err
	}

	return obj.(*oauthapi.OAuthClient), err
}
