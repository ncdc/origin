package testclient

import (
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"k8s.io/kubernetes/pkg/client/testing/core"
)

// FakeImageSignatures implements ImageSignatureInterface. Meant to
// be embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageSignatures struct {
	Fake *Fake
}

var _ client.ImageSignatureInterface = &FakeImageSignatures{}

func (c *FakeImageSignatures) Create(inObj *imageapi.ImageSignature) (*imageapi.ImageSignature, error) {
	_, err := c.Fake.Invokes(core.NewRootCreateAction(imageapi.SchemeGroupVersion.WithResource("imagesignatures"), inObj), inObj)
	return inObj, err
}

func (c *FakeImageSignatures) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewRootDeleteAction(imageapi.SchemeGroupVersion.WithResource("imagesignatures"), name), &imageapi.ImageSignature{})
	return err
}
