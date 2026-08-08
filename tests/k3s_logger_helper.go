package tests

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"sync"
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
	cancel    context.CancelFunc
}

type NamespaceLogger struct {
	client    *kubernetes.Clientset
	namespace string
	logger    *log.Logger
	factory   informers.SharedInformerFactory
	informer  cache.SharedIndexInformer
	loggers   map[string]*PodLogger
	lock      sync.Mutex
	ctx       context.Context
}

func NewNamespaceLogger(client *kubernetes.Clientset, namespace string, logger *log.Logger) *NamespaceLogger {
	factory := informers.NewSharedInformerFactoryWithOptions(client, time.Second*5, informers.WithNamespace(namespace))

	return &NamespaceLogger{
		client:    client,
		namespace: namespace,
		logger:    logger,
		factory:   factory,
		informer:  factory.Core().V1().Pods().Informer(),
		loggers:   make(map[string]*PodLogger),
	}
}

func (nl *NamespaceLogger) Start(ctx context.Context) {
	nl.ctx = ctx
	nl.logger.Printf("Starting pod logger for namespace: %s", nl.namespace)

	nl.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nl.handlePodAdd,
		UpdateFunc: nl.handlePodUpdate,
		DeleteFunc: nl.handlePodDelete,
	})

	go nl.factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), nl.informer.HasSynced) {
		nl.logger.Printf("informer sync failed or context was cancelled")
		return
	}
	nl.logger.Printf("Pod logger started for namespace: %s", nl.namespace)
	<-ctx.Done()
}

func (nl *NamespaceLogger) handlePodAdd(obj interface{}) {
	pod := obj.(*v1.Pod)
	if canStartLogging(pod) {
		nl.startPodLogger(pod.Name)
	}
}

func (nl *NamespaceLogger) handlePodUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*v1.Pod)
	newPod := newObj.(*v1.Pod)

	if canStartLogging(newPod) && !canStartLogging(oldPod) {
		nl.logger.Printf("Attached Logger for %s/%s\n", newPod.Namespace, newPod.Name)
		nl.startPodLogger(newPod.Name)
	}
}

func (nl *NamespaceLogger) handlePodDelete(obj interface{}) {
	pod := obj.(*v1.Pod)
	nl.logger.Printf("Pod Removed: %s/%s\n", pod.Namespace, pod.Name)

	nl.lock.Lock()
	defer nl.lock.Unlock()
	if _, exists := nl.loggers[pod.Name]; exists {
		nl.logger.Printf("Stopped logging for pod: %s/%s\n", pod.Namespace, pod.Name)
		nl.loggers[pod.Name].StopLogging()
		delete(nl.loggers, pod.Name)
	}
}

func (nl *NamespaceLogger) startPodLogger(podname string) {
	nl.lock.Lock()
	defer nl.lock.Unlock()

	if _, exists := nl.loggers[podname]; exists {
		return
	}

	podLogger := NewPodLogger(nl.client, nl.namespace, podname, nl.logger, nl.ctx)
	nl.loggers[podname] = podLogger
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

func NewPodLogger(client *kubernetes.Clientset, namespace, podname string, baseline *log.Logger, ctx context.Context) *PodLogger {
	ctx, cancel := context.WithCancel(ctx)
	logger := log.New(baseline.Writer(), fmt.Sprintf("[%s/%s] ", namespace, podname), baseline.Flags())
	return &PodLogger{
		client:    client,
		namespace: namespace,
		podname:   podname,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (pl *PodLogger) StopLogging() {
	pl.cancel()
}

func (pl *PodLogger) StartLogging() {
	defer pl.cancel()

	podLogOptions := &v1.PodLogOptions{
		Follow:     true,
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
