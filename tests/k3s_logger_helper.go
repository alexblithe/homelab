package tests

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
)

type PodLogger struct {
	client    *kubernetes.Clientset
	namespace string
	podname   string
	tmpDir    string
	ctx       context.Context
}

func NewPodLogger(client *kubernetes.Clientset, namespace, podname, tmpDir string, ctx context.Context) *PodLogger {
	return &PodLogger{
		client:    client,
		namespace: namespace,
		podname:   podname,
		tmpDir:    tmpDir,
		ctx:       ctx,
	}
}

func (pl *PodLogger) StartLogging() {
	podLogOptions := &v1.PodLogOptions{
		Follow:     false,
		Timestamps: true,
	}

	req := pl.client.CoreV1().Pods(pl.namespace).GetLogs(pl.podname, podLogOptions)

	logFilePath := filepath.Join(pl.tmpDir, pl.podname+".log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Fatalf("failed to create log file for pod %s: %v", pl.podname, err)
	}
	defer logFile.Close()

	podLogs, err := req.Stream(pl.ctx)
	if err != nil {
		log.Fatalf("failed to get logs for pod %s: %v", pl.podname, err)
	}
	defer podLogs.Close()

	_, err = io.Copy(logFile, podLogs)
	if err != nil {
		log.Fatalf("failed to write logs for pod %s to file: %v", pl.podname, err)
	}

	log.Printf("Logs for pod %s written to %s", pl.podname, logFilePath)
}

func InstallLogger(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(client, time.Second*5, informers.WithNamespace(namespace))
	informer := factory.Core().V1().Pods().Informer()

	loggersMap := make(map[string]*PodLogger)
	lock := sync.Mutex{}
	log.Printf("Starting pod logger for namespace: %s", namespace)
	if err := os.MkdirAll("../logs/", os.ModePerm); err != nil {
		t.Fatalf("failed to create logs directory: %v", err)
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if canStartLogging(pod) {
				startPodLogger(loggersMap, &lock, client, namespace, pod.Name, ctx)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*v1.Pod)
			newPod := newObj.(*v1.Pod)

			if canStartLogging(newPod) && !canStartLogging(oldPod) {
				log.Printf("Pod can produce logs: %s/%s\n", newPod.Namespace, newPod.Name)
				startPodLogger(loggersMap, &lock, client, namespace, newPod.Name, ctx)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			log.Printf("Pod Removed: %s/%s\n", pod.Namespace, pod.Name)
			lock.Lock()
			defer lock.Unlock()
			if _, exists := loggersMap[pod.Name]; exists {
				log.Printf("Stopped logging for pod: %s/%s\n", pod.Namespace, pod.Name)
				delete(loggersMap, pod.Name)
			}
		},
	})

	log.Printf("starting informer")
	go factory.Start(ctx.Done())

	log.Printf("waiting for informer sync")
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Printf("informer sync failed or context was cancelled")
		return
	}
	log.Printf("informer sync successful")
	<-ctx.Done()

}

func startPodLogger(loggersMap map[string]*PodLogger, lock *sync.Mutex, client *kubernetes.Clientset, namespace, podname string, ctx context.Context) {
	lock.Lock()
	defer lock.Unlock()

	if _, exists := loggersMap[podname]; exists {
		return
	}

	logger := NewPodLogger(client, namespace, podname, "../logs", ctx)
	loggersMap[podname] = logger
	go logger.StartLogging()
}

func canStartLogging(pod *v1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil {
			return true
		}
	}
	return false
}
