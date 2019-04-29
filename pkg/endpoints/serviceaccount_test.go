package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/satori/go.uuid"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var server *httptest.Server
const namespace string = "fake"

func TestMain(m *testing.M) {
	wsContainer := restful.NewContainer()

	resource := dummyResource()
	resource.K8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})

	pipeline := v1alpha1.Pipeline{}
	pipeline.Name = "fakepipeline"

	createdPipeline, err := resource.PipelineClient.TektonV1alpha1().Pipelines(namespace).Create(&pipeline)
}

func TestPatchServiceAccount(t *Testing) {

}