/*
Copyright 2019 The Tekton Authors
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

package main

import (
	restful "github.com/emicklei/go-restful"
	kubecontroller "github.com/tektoncd/dashboard/pkg/controllers/kubernetes"
	tektoncontroller "github.com/tektoncd/dashboard/pkg/controllers/tekton"
	"github.com/tektoncd/dashboard/pkg/endpoints"
	logging "github.com/tektoncd/dashboard/pkg/logging"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektoninformers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	k8sinformers "k8s.io/client-go/informers"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"
	"net/http"
	"os"
	"time"
)

// Stores config env
type config struct {
	kubeConfigPath string
	// Should conform with http.Server.Addr field
	port             string
	installNamespace string
}

func main() {
	dashboardConfig := config{
		kubeConfigPath:   os.Getenv("KUBECONFIG"),
		port:             ":8080",
		installNamespace: os.Getenv("INSTALLED_NAMESPACE"),
	}

	var cfg *rest.Config
	var err error
	if len(dashboardConfig.kubeConfigPath) != 0 {
		cfg, err = clientcmd.BuildConfigFromFlags("", dashboardConfig.kubeConfigPath)
		if err != nil {
			logging.Log.Errorf("Error building kubeconfig from %s: %s", dashboardConfig.kubeConfigPath, err.Error())
		}
	} else {
		if cfg, err = rest.InClusterConfig(); err != nil {
			logging.Log.Errorf("Error building kubeconfig: %s", err.Error())
		}
	}

	portNumber := os.Getenv("PORT")
	if portNumber != "" {
		dashboardConfig.port = ":" + portNumber
		logging.Log.Infof("Port number from config: %s", portNumber)
	}

	wsContainer := restful.NewContainer()
	wsContainer.Router(restful.CurlyRouter{})

	pipelineClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logging.Log.Errorf("Error building pipeline clientset: %s", err.Error())
	}

	k8sClient, err := k8sclientset.NewForConfig(cfg)
	if err != nil {
		logging.Log.Errorf("Error building k8s clientset: %s", err.Error())
	}

	resource := endpoints.Resource{
		PipelineClient: pipelineClient,
		K8sClient:      k8sClient,
	}

	// Any functional changes to the the router mux should be reflected in endpoints/test-utils.test_go/func dummyServer()
	logging.Log.Info("Registering REST endpoints")
	resource.RegisterWeb(wsContainer)
	resource.RegisterEndpoints(wsContainer)
	resource.RegisterWebsocket(wsContainer)
	resource.RegisterHealthProbe(wsContainer)
	resource.RegisterReadinessProbe(wsContainer)
	endpoints.RegisterExtensions(wsContainer)

	logging.Log.Info("Creating controllers")
	stopCh := signals.SetupSignalHandler()
	resyncDur := time.Second * 30
	tektonInformerFactory := tektoninformers.NewSharedInformerFactory(resource.PipelineClient, resyncDur)
	kubeInformerFactory := k8sinformers.NewSharedInformerFactory(resource.K8sClient, resyncDur)
	// Add all tekton controllers
	tektoncontroller.NewTaskRunController(tektonInformerFactory, stopCh)
	tektoncontroller.NewPipelineRunController(tektonInformerFactory, stopCh)
	// Add all kube controllers
	kubecontroller.NewExtensionController(kubeInformerFactory, stopCh, wsContainer)
	kubecontroller.NewNamespaceController(kubeInformerFactory, stopCh)
	// Started once all controllers have been registered
	logging.Log.Info("Starting controllers")
	tektonInformerFactory.Start(stopCh)
	kubeInformerFactory.Start(stopCh)

	logging.Log.Infof("Creating server and entering wait loop")
	server := &http.Server{Addr: dashboardConfig.port, Handler: wsContainer}
	logging.Log.Fatal(server.ListenAndServe())
}
