package kubernetes

import (
	restful "github.com/emicklei/go-restful"
	"github.com/tektoncd/dashboard/pkg/endpoints"
	logging "github.com/tektoncd/dashboard/pkg/logging"
	v1 "k8s.io/api/core/v1"
	k8sinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// Registers the K8s shared informer that reacts to extension service updates
func NewExtensionController(sharedK8sInformerFactory k8sinformer.SharedInformerFactory, stopCh <-chan struct{}, mux *restful.Container) {
	logging.Log.Debug("Into StartExtensionController")
	extensionServiceInformer := sharedK8sInformerFactory.Core().V1().Services().Informer()
	// ResourceEventHandler interface functions only pass object interfaces
	// Inlined functions to keep mux in scope rather than reconciler
	extensionServiceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*v1.Service)

			if value := service.Labels[endpoints.ExtensionLabelKey]; value == endpoints.ExtensionLabelValue {
				endpoints.RegisterExtension(mux, service)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldService, newService := oldObj.(*v1.Service), newObj.(*v1.Service)
			// If resourceVersion differs between old and new, an actual update event was observed
			if value := newService.Labels[endpoints.ExtensionLabelKey]; value == endpoints.ExtensionLabelValue &&
				oldService.ResourceVersion != newService.ResourceVersion {
				endpoints.UnregisterExtension(mux, oldService)
				endpoints.RegisterExtension(mux, newService)
			}
		},
		DeleteFunc: func(obj interface{}) {
			service := obj.(*v1.Service)
			if value := service.Labels[endpoints.ExtensionLabelKey]; value == endpoints.ExtensionLabelValue {
				endpoints.UnregisterExtension(mux, service)
			}
		},
	})
}
