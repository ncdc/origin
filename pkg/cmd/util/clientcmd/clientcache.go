package clientcmd

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
)

// clientCache caches previously loaded clients for reuse. This is largely
// copied from upstream (because of typing) but reuses the negotiation logic.
// TODO: Consolidate this entire concept with upstream's ClientCache.
type clientCache struct {
	loader        kclientcmd.ClientConfig
	clients       map[string]*client.Client
	configs       map[string]*restclient.Config
	defaultConfig *restclient.Config
	// negotiatingClient is used only for negotiating versions with the server.
	negotiatingClient restclient.Interface
}

// ClientConfigForVersion returns the correct config for a server
func (c *clientCache) ClientConfigForVersion(version *unversioned.GroupVersion) (*restclient.Config, error) {
	if c.defaultConfig == nil {
		config, err := c.loader.ClientConfig()
		if err != nil {
			return nil, err
		}
		c.defaultConfig = config
	}
	// TODO: have a better config copy method
	cacheKey := ""
	if version != nil {
		cacheKey = version.String()
	}
	if config, ok := c.configs[cacheKey]; ok {
		return config, nil
	}
	if c.negotiatingClient == nil {
		// TODO: We want to reuse the upstream negotiation logic, which is coupled
		// to a concrete kube Client. The negotiation will ultimately try and
		// build an unversioned URL using the config prefix to ask for supported
		// server versions. If we use the default kube client config, the prefix
		// will be /api, while we need to use the OpenShift prefix to ask for the
		// OpenShift server versions. For now, set OpenShift defaults on the
		// config to ensure the right prefix gets used. The client cache and
		// negotiation logic should be refactored upstream to support downstream
		// reuse so that we don't need to do any of this cache or negotiation
		// duplication.
		negotiatingConfig := *c.defaultConfig
		client.SetOpenShiftDefaults(&negotiatingConfig)
		negotiatingClient, err := restclient.RESTClientFor(&negotiatingConfig)
		if err != nil {
			return nil, err
		}
		c.negotiatingClient = negotiatingClient
	}
	config := *c.defaultConfig
	negotiatedVersion, err := negotiateVersion(c.negotiatingClient, &config, version, latest.Versions)
	if err != nil {
		return nil, err
	}
	config.GroupVersion = negotiatedVersion
	client.SetOpenShiftDefaults(&config)
	c.configs[cacheKey] = &config

	// `version` does not necessarily equal `config.Version`.  However, we know that we call this method again with
	// `config.Version`, we should get the the config we've just built.
	configCopy := config
	c.configs[config.GroupVersion.String()] = &configCopy

	return &config, nil
}

// ClientForVersion initializes or reuses a client for the specified version, or returns an
// error if that is not possible
func (c *clientCache) ClientForVersion(version *unversioned.GroupVersion) (*client.Client, error) {
	cacheKey := ""
	if version != nil {
		cacheKey = version.String()
	}
	if client, ok := c.clients[cacheKey]; ok {
		return client, nil
	}
	config, err := c.ClientConfigForVersion(version)
	if err != nil {
		return nil, err
	}
	client, err := client.New(config)
	if err != nil {
		return nil, err
	}

	c.clients[config.GroupVersion.String()] = client
	return client, nil
}
