// This file was automatically generated by lister-gen with arguments: --input-dirs=[github.com/openshift/origin/pkg/authorization/api,github.com/openshift/origin/pkg/authorization/api/v1,github.com/openshift/origin/pkg/build/api,github.com/openshift/origin/pkg/build/api/v1,github.com/openshift/origin/pkg/deploy/api,github.com/openshift/origin/pkg/deploy/api/v1,github.com/openshift/origin/pkg/image/api,github.com/openshift/origin/pkg/image/api/v1,github.com/openshift/origin/pkg/oauth/api,github.com/openshift/origin/pkg/oauth/api/v1,github.com/openshift/origin/pkg/project/api,github.com/openshift/origin/pkg/project/api/v1,github.com/openshift/origin/pkg/route/api,github.com/openshift/origin/pkg/route/api/v1,github.com/openshift/origin/pkg/sdn/api,github.com/openshift/origin/pkg/sdn/api/v1,github.com/openshift/origin/pkg/template/api,github.com/openshift/origin/pkg/template/api/v1,github.com/openshift/origin/pkg/user/api,github.com/openshift/origin/pkg/user/api/v1] --logtostderr=true --output-base=/home/vagrant/go/src

package v1

import (
	api "github.com/openshift/origin/pkg/template/api"
	v1 "github.com/openshift/origin/pkg/template/api/v1"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"
)

// TemplateLister helps list Templates.
type TemplateLister interface {
	// List lists all Templates in the indexer.
	List(selector labels.Selector) (ret []*v1.Template, err error)
	// Templates returns an object that can list and get Templates.
	Templates(namespace string) TemplateNamespaceLister
	TemplateListerExpansion
}

// templateLister implements the TemplateLister interface.
type templateLister struct {
	indexer cache.Indexer
}

// NewTemplateLister returns a new TemplateLister.
func NewTemplateLister(indexer cache.Indexer) TemplateLister {
	return &templateLister{indexer: indexer}
}

// List lists all Templates in the indexer.
func (s *templateLister) List(selector labels.Selector) (ret []*v1.Template, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Template))
	})
	return ret, err
}

// Templates returns an object that can list and get Templates.
func (s *templateLister) Templates(namespace string) TemplateNamespaceLister {
	return templateNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// TemplateNamespaceLister helps list and get Templates.
type TemplateNamespaceLister interface {
	// List lists all Templates in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1.Template, err error)
	// Get retrieves the Template from the indexer for a given namespace and name.
	Get(name string) (*v1.Template, error)
	TemplateNamespaceListerExpansion
}

// templateNamespaceLister implements the TemplateNamespaceLister
// interface.
type templateNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Templates in the indexer for a given namespace.
func (s templateNamespaceLister) List(selector labels.Selector) (ret []*v1.Template, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.Template))
	})
	return ret, err
}

// Get retrieves the Template from the indexer for a given namespace and name.
func (s templateNamespaceLister) Get(name string) (*v1.Template, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(api.Resource("template"), name)
	}
	return obj.(*v1.Template), nil
}
