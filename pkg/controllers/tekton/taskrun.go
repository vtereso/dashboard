package tekton

import (
	"github.com/tektoncd/dashboard/pkg/broadcaster"
	"github.com/tektoncd/dashboard/pkg/endpoints"
	logging "github.com/tektoncd/dashboard/pkg/logging"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tektoninformer "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
)

// Registers a Tekton controller/informer for taskRuns on sharedTektonInformerFactory
func NewTaskRunController(sharedTektonInformerFactory tektoninformer.SharedInformerFactory, stopCh <-chan struct{}) {
	logging.Log.Debug("Into StartTaskRunController")
	taskRunInformer := sharedTektonInformerFactory.Tekton().V1alpha1().TaskRuns().Informer()
	taskRunInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    taskRunCreated,
		UpdateFunc: taskRunUpdated,
		DeleteFunc: taskRunDeleted,
	})
}

func taskRunCreated(obj interface{}) {
	logging.Log.Debug("In taskRunCreated")
	data := broadcaster.SocketData{
		MessageType: broadcaster.TaskRunCreated,
		Payload:     obj.(*v1alpha1.TaskRun),
	}
	endpoints.ResourcesChannel <- data
}

func taskRunUpdated(oldObj, newObj interface{}) {
	logging.Log.Debug("In taskRunUpdated")
	if newObj.(*v1alpha1.TaskRun).GetResourceVersion() != oldObj.(*v1alpha1.TaskRun).GetResourceVersion() {
		data := broadcaster.SocketData{
			MessageType: broadcaster.TaskRunUpdated,
			Payload:     newObj.(*v1alpha1.TaskRun),
		}
		endpoints.ResourcesChannel <- data
	}
}

func taskRunDeleted(obj interface{}) {
	logging.Log.Debug("In taskRunDeleted")
	data := broadcaster.SocketData{
		MessageType: broadcaster.TaskRunDeleted,
		Payload:     obj.(*v1alpha1.TaskRun),
	}
	endpoints.ResourcesChannel <- data
}
