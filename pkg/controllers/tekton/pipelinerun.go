package tekton

import (
	"github.com/tektoncd/dashboard/pkg/broadcaster"
	"github.com/tektoncd/dashboard/pkg/endpoints"
	logging "github.com/tektoncd/dashboard/pkg/logging"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tektoninformer "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
)

// Registers a Tekton controller/informer for pipelineRuns on sharedTektonInformerFactory
func NewPipelineRunController(sharedTektonInformerFactory tektoninformer.SharedInformerFactory, stopCh <-chan struct{}) {
	logging.Log.Debug("Into StartPipelineRunController")
	pipelineRunInformer := sharedTektonInformerFactory.Tekton().V1alpha1().PipelineRuns().Informer()
	pipelineRunInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pipelineRunCreated,
		UpdateFunc: pipelineRunUpdated,
		DeleteFunc: pipelineRunDeleted,
	})
}

//pipeline run events
func pipelineRunCreated(obj interface{}) {
	logging.Log.Debug("In pipelineRunCreated")
	data := broadcaster.SocketData{
		MessageType: broadcaster.PipelineRunCreated,
		Payload:     obj.(*v1alpha1.PipelineRun),
	}
	endpoints.ResourcesChannel <- data
}

func pipelineRunUpdated(oldObj, newObj interface{}) {
	logging.Log.Debug("In pipelineRunUpdated")
	if newObj.(*v1alpha1.PipelineRun).GetResourceVersion() != oldObj.(*v1alpha1.PipelineRun).GetResourceVersion() {
		data := broadcaster.SocketData{
			MessageType: broadcaster.PipelineRunUpdated,
			Payload:     newObj.(*v1alpha1.PipelineRun),
		}
		endpoints.ResourcesChannel <- data
	}
}

func pipelineRunDeleted(obj interface{}) {
	logging.Log.Debug("In pipelineRunDeleted")
	data := broadcaster.SocketData{
		MessageType: broadcaster.PipelineRunDeleted,
		Payload:     obj.(*v1alpha1.PipelineRun),
	}
	endpoints.ResourcesChannel <- data
}
