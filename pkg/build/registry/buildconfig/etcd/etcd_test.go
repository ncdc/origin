package etcd

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"

	"github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	"github.com/openshift/origin/pkg/build/registry/buildconfig"
	"github.com/openshift/origin/pkg/util/restoptions"
)

func newStorage(t *testing.T) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	storage, err := NewREST(restoptions.NewSimpleGetter(etcdStorage))
	if err != nil {
		t.Fatal(err)
	}
	return storage, server
}

func TestStorage(t *testing.T) {
	storage, _ := newStorage(t)
	buildconfig.NewRegistry(storage)
}

func validBuildConfig() *api.BuildConfig {
	return &api.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "configid"},
		Spec: api.BuildConfigSpec{
			RunPolicy: api.BuildRunPolicySerial,
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: api.BuildStrategy{
					DockerStrategy: &api.DockerBuildStrategy{},
				},
				Output: api.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	valid := validBuildConfig()
	valid.Name = ""
	valid.GenerateName = "test-"
	test.TestCreate(
		valid,
		// invalid
		&api.BuildConfig{},
	)
}

func TestUpdate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestUpdate(
		validBuildConfig(),
		// updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.BuildConfig)
			object.Spec.CommonSpec.Source.Git.URI = "http://github.com/my/otherrepo"
			return object
		},
		// invalid updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.BuildConfig)
			object.Spec.CommonSpec.Source.Git.URI = ""
			return object
		},
	)
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestList(
		validBuildConfig(),
	)
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestGet(
		validBuildConfig(),
	)
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestDelete(
		validBuildConfig(),
	)
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)

	valid := validBuildConfig()
	valid.Name = "foo"
	valid.Labels = map[string]string{"foo": "bar"}

	test.TestWatch(
		valid,
		// matching labels
		[]labels.Set{{"foo": "bar"}},
		// not matching labels
		[]labels.Set{{"foo": "baz"}},
		// matching fields
		[]fields.Set{
			{"metadata.name": "foo"},
		},
		// not matching fields
		[]fields.Set{
			{"metadata.name": "bar"},
		},
	)
}
