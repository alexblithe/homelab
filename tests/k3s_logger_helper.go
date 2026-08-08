package tests

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/kubernetes"
)

type PodLogger struct {
	client    *kubernetes.Clientset
	namespace string
	podname   string
	logger    *log.Logger
	ctx       context.Context
}

func NewPodLogger(client *kubernetes.Clientset, namespace, podname string, baseline *log.Logger, ctx context.Context) *PodLogger {
	logger := log.New(baseline.Writer(), fmt.Sprintf("[%s/%s] ", namespace, podname), baseline.Flags())
	return &PodLogger{
		client:    client,
		namespace: namespace,
		podname:   podname,
		logger:    logger,
		ctx:       ctx,
	}
}

func (pl *PodLogger) StartLogging() {
	podLogOptions := &v1.PodLogOptions{
		Follow:     false,
		Timestamps: true,
	}

	req := pl.client.CoreV1().Pods(pl.namespace).GetLogs(pl.podname, podLogOptions)

	podLogs, err := req.Stream(pl.ctx)
	if err != nil {
		pl.logger.Printf("failed to get logs for pod %s/%s: %v", pl.namespace, pl.podname, err)
		return
	}
	defer podLogs.Close()

	scanner := bufio.NewScanner(podLogs)
	for scanner.Scan() {
		pl.logger.Printf("%s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		pl.logger.Printf("failed to read logs for pod %s/%s: %v", pl.namespace, pl.podname, err)
	}
}

func InstallLogger(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace string, logger *log.Logger) {
	factory := informers.NewSharedInformerFactoryWithOptions(client, time.Second*5, informers.WithNamespace(namespace))
	informer := factory.Core().V1().Pods().Informer()

	loggersMap := make(map[string]*PodLogger)
	lock := sync.Mutex{}
	logger.Printf("Starting pod logger for namespace: %s", namespace)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if canStartLogging(pod) {
				startPodLogger(loggersMap, &lock, client, namespace, pod.Name, logger, ctx)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*v1.Pod)
			newPod := newObj.(*v1.Pod)

			if canStartLogging(newPod) && !canStartLogging(oldPod) {
				logger.Printf("Attached Logger for %s/%s\n", newPod.Namespace, newPod.Name)
				startPodLogger(loggersMap, &lock, client, namespace, newPod.Name, logger, ctx)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			logger.Printf("Pod Removed: %s/%s\n", pod.Namespace, pod.Name)
			lock.Lock()
			defer lock.Unlock()
			if _, exists := loggersMap[pod.Name]; exists {
				logger.Printf("Stopped logging for pod: %s/%s\n", pod.Namespace, pod.Name)
				delete(loggersMap, pod.Name)
			}
		},
	})

	go factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		logger.Printf("informer sync failed or context was cancelled")
		return
	}
	logger.Printf("Pod logger started for namespace: %s", namespace)
	<-ctx.Done()

}

func startPodLogger(loggersMap map[string]*PodLogger, lock *sync.Mutex, client *kubernetes.Clientset, namespace, podname string, logger *log.Logger, ctx context.Context) {
	lock.Lock()
	defer lock.Unlock()

	if _, exists := loggersMap[podname]; exists {
		return
	}

	podLogger := NewPodLogger(client, namespace, podname, logger, ctx)
	loggersMap[podname] = podLogger
	go podLogger.StartLogging()
}

func canStartLogging(pod *v1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil {
			return true
		}
	}
	return false
}
