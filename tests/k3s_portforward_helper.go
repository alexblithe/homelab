package tests

import (
	"context"
	"fmt"
	"io"
	"net/http"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwarder struct {
	restConfig *rest.Config
	client     kubernetes.Interface
	namespace  string
	service    string
	localPort  int
	targetPort int

	cancel context.CancelFunc
}

func NewPortForwarder(restConfig *rest.Config, client kubernetes.Interface, namespace, service string, localPort, targetPort int) *PortForwarder {
	return &PortForwarder{
		restConfig: restConfig,
		client:     client,
		namespace:  namespace,
		service:    service,
		localPort:  localPort,
		targetPort: targetPort,
	}
}

func (pf *PortForwarder) Start(ctx context.Context) error {
	pod, err := pf.findPod(ctx)
	if err != nil {
		return err
	}

	transport, upgrader, err := spdy.RoundTripperFor(pf.restConfig)
	if err != nil {
		return err
	}

	url := pf.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pf.namespace).
		Name(pod.Name).
		SubResource("portforward").
		URL()
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)

	var forwardCtx context.Context
	forwardCtx, pf.cancel = context.WithCancel(ctx)
	ready := make(chan struct{})
	forwarder, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", pf.localPort, pf.targetPort)},
		forwardCtx.Done(),
		ready,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		pf.cancel()
		return err
	}

	forwardErr := make(chan error, 1)
	go func() { forwardErr <- forwarder.ForwardPorts() }()

	select {
	case <-ready:
		return nil
	case err := <-forwardErr:
		pf.cancel()
		return err
	case <-ctx.Done():
		pf.cancel()
		return ctx.Err()
	}
}

func (pf *PortForwarder) Stop() {
	if pf.cancel != nil {
		pf.cancel()
	}
}

func (pf *PortForwarder) findPod(ctx context.Context) (*v1.Pod, error) {
	service, err := pf.client.CoreV1().Services(pf.namespace).Get(ctx, pf.service, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if len(service.Spec.Selector) == 0 {
		return nil, fmt.Errorf("service %q has no selector", pf.service)
	}

	pods, err := pf.client.CoreV1().Pods(pf.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(service.Spec.Selector).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == v1.PodRunning && podReady(pod) {
			return pod, nil
		}
	}

	return nil, fmt.Errorf("service %q has no running, ready pods", pf.service)
}

func podReady(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}
