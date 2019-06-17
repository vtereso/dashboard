package kubernetes

import (
	"github.com/tektoncd/dashboard/pkg/broadcaster"
	"github.com/tektoncd/dashboard/pkg/endpoints"
	logging "github.com/tektoncd/dashboard/pkg/logging"
	v1 "k8s.io/api/core/v1"
	k8sinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func NewNamespaceController(sharedK8sInformerFactory k8sinformer.SharedInformerFactory, stopCh <-chan struct{}) {
	logging.Log.Debug("Into StartResourcesController")
	k8sInformer := sharedK8sInformerFactory.Core().V1().Namespaces()
	k8sInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    namespaceCreated,
		DeleteFunc: namespaceDeleted,
	})
}

func namespaceCreated(obj interface{}) {
	logging.Log.Debug("In namespaceCreated")
	data := broadcaster.SocketData{
		MessageType: broadcaster.NamespaceCreated,
		Payload:     obj.(*v1.Namespace),
	}

	endpoints.ResourcesChannel <- data
}

func namespaceDeleted(obj interface{}) {
	logging.Log.Debug("In namespaceDeleted")
	data := broadcaster.SocketData{
		MessageType: broadcaster.NamespaceDeleted,
		Payload:     obj.(*v1.Namespace),
	}

	endpoints.ResourcesChannel <- data
}
