/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dockertools

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	kubecontainer "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/container"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/network"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/prober"
	kubeletTypes "github.com/GoogleCloudPlatform/kubernetes/pkg/kubelet/types"
)

func NewFakeDockerManager(
	client DockerInterface,
	recorder record.EventRecorder,
	readinessManager *kubecontainer.ReadinessManager,
	containerRefManager *kubecontainer.RefManager,
	podInfraContainerImage string,
	qps float32,
	burst int,
	containerLogsDir string,
	osInterface kubecontainer.OSInterface,
	networkPlugin network.NetworkPlugin,
	generator kubecontainer.RunContainerOptionsGenerator,
	httpClient kubeletTypes.HttpGetter,
	runtimeHooks kubecontainer.RuntimeHooks) *DockerManager {

	dm := NewDockerManager(client, recorder, readinessManager, containerRefManager, podInfraContainerImage, qps,
		burst, containerLogsDir, osInterface, networkPlugin, generator, httpClient, runtimeHooks, "")
	dm.Puller = &FakeDockerPuller{}
	dm.prober = prober.New(nil, readinessManager, containerRefManager, recorder)
	return dm
}
