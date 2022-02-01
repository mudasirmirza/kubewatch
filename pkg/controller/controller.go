/*
Copyright 2016 Skippbox, Ltd.

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

package controller

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mudasirmirza/kubewatch/config"
	"github.com/mudasirmirza/kubewatch/pkg/event"
	"github.com/mudasirmirza/kubewatch/pkg/handlers"
	"github.com/mudasirmirza/kubewatch/pkg/utils"

	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	batch_v1 "k8s.io/api/batch/v1"
	api_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const maxRetries = 5

var serverStartTime time.Time

// Maps for holding events config
var global map[string]uint8
var create map[string]uint8
var delete map[string]uint8
var update map[string]uint8

// Event indicate the informerEvent
type Event struct {
	key          string
	eventType    string
	namespace    string
	resourceType string
}

// Controller object
type Controller struct {
	logger       *logrus.Entry
	clientset    kubernetes.Interface
	queue        workqueue.RateLimitingInterface
	informer     cache.SharedIndexInformer
	eventHandler handlers.Handler
}

// Start prepares watchers and run their controllers, then waits for process termination signals
func Start(conf *config.Config, eventHandler handlers.Handler) {

	// loads events config into memory for granular alerting
	loadEventConfig(conf)

	var kubeClient kubernetes.Interface
	_, err := rest.InClusterConfig()
	if err != nil {
		kubeClient = utils.GetClientOutOfCluster()
	} else {
		kubeClient = utils.GetClient()
	}

	if len(conf.Namespace) == 0 {
		conf.Namespace = append(conf.Namespace, "")
	}

	if conf.Resource.Pod {
		for _, ns := range conf.Namespace {
			fmt.Println(ns)
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.CoreV1().Pods(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.CoreV1().Pods(ns).Watch(options)
					},
				},
				&api_v1.Pod{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "pod")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.DaemonSet {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.ExtensionsV1beta1().DaemonSets(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.ExtensionsV1beta1().DaemonSets(ns).Watch(options)
					},
				},
				&ext_v1beta1.DaemonSet{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "daemonset")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.ReplicaSet {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.ExtensionsV1beta1().ReplicaSets(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.ExtensionsV1beta1().ReplicaSets(ns).Watch(options)
					},
				},
				&ext_v1beta1.ReplicaSet{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "replicaset")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.Service {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.CoreV1().Services(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.CoreV1().Services(ns).Watch(options)
					},
				},
				&api_v1.Service{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "service")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.Deployment {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.AppsV1beta1().Deployments(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.AppsV1beta1().Deployments(ns).Watch(options)
					},
				},
				&apps_v1beta1.Deployment{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "deployment")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.Namespace {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().Namespaces().List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().Namespaces().Watch(options)
				},
			},
			&api_v1.Namespace{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(kubeClient, eventHandler, informer, "namespace")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.ReplicationController {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.CoreV1().ReplicationControllers(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.CoreV1().ReplicationControllers(ns).Watch(options)
					},
				},
				&api_v1.ReplicationController{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "replication controller")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.Job {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.BatchV1().Jobs(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.BatchV1().Jobs(ns).Watch(options)
					},
				},
				&batch_v1.Job{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "job")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.PersistentVolume {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().PersistentVolumes().List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().PersistentVolumes().Watch(options)
				},
			},
			&api_v1.PersistentVolume{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(kubeClient, eventHandler, informer, "persistent volume")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Secret {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.CoreV1().Secrets(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.CoreV1().Secrets(ns).Watch(options)
					},
				},
				&api_v1.Secret{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "secret")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.ConfigMap {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.CoreV1().ConfigMaps(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.CoreV1().ConfigMaps(ns).Watch(options)
					},
				},
				&api_v1.ConfigMap{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "configmap")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	if conf.Resource.Ingress {
		for _, ns := range conf.Namespace {
			informer := cache.NewSharedIndexInformer(
				&cache.ListWatch{
					ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
						return kubeClient.ExtensionsV1beta1().Ingresses(ns).List(options)
					},
					WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
						return kubeClient.ExtensionsV1beta1().Ingresses(ns).Watch(options)
					},
				},
				&ext_v1beta1.Ingress{},
				0, //Skip resync
				cache.Indexers{},
			)

			c := newResourceController(kubeClient, eventHandler, informer, "ingress")
			stopCh := make(chan struct{})
			defer close(stopCh)

			go c.Run(stopCh)
		}
	}

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func newResourceController(client kubernetes.Interface, eventHandler handlers.Handler, informer cache.SharedIndexInformer, resourceType string) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var newEvent Event
	var err error
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			newEvent.key, err = cache.MetaNamespaceKeyFunc(obj)
			newEvent.eventType = "create"
			newEvent.resourceType = resourceType
			logrus.WithField("pkg", "kubewatch-"+resourceType).Infof("Processing add to %v: %s", resourceType, newEvent.key)
			if err == nil {
				queue.Add(newEvent)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			newEvent.key, err = cache.MetaNamespaceKeyFunc(old)
			newEvent.eventType = "update"
			newEvent.resourceType = resourceType
			logrus.WithField("pkg", "kubewatch-"+resourceType).Infof("Processing update to %v: %s", resourceType, newEvent.key)
			if err == nil {
				queue.Add(newEvent)
			}
		},
		DeleteFunc: func(obj interface{}) {
			newEvent.key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			newEvent.eventType = "delete"
			newEvent.resourceType = resourceType
			newEvent.namespace = utils.GetObjectMetaData(obj).Namespace
			logrus.WithField("pkg", "kubewatch-"+resourceType).Infof("Processing delete to %v: %s", resourceType, newEvent.key)
			if err == nil {
				queue.Add(newEvent)
			}
		},
	})

	return &Controller{
		logger:       logrus.WithField("pkg", "kubewatch-"+resourceType),
		clientset:    client,
		informer:     informer,
		queue:        queue,
		eventHandler: eventHandler,
	}
}

// Run starts the kubewatch controller
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	c.logger.Info("Starting kubewatch controller")
	serverStartTime = time.Now().Local()

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	c.logger.Info("Kubewatch controller synced and ready")

	wait.Until(c.runWorker, time.Second, stopCh)
}

// HasSynced is required for the cache.Controller interface.
func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

// LastSyncResourceVersion is required for the cache.Controller interface.
func (c *Controller) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *Controller) processNextItem() bool {
	newEvent, quit := c.queue.Get()

	if quit {
		return false
	}
	defer c.queue.Done(newEvent)
	err := c.processItem(newEvent.(Event))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(newEvent)
	} else if c.queue.NumRequeues(newEvent) < maxRetries {
		c.logger.Errorf("Error processing %s (will retry): %v", newEvent.(Event).key, err)
		c.queue.AddRateLimited(newEvent)
	} else {
		// err != nil and too many retries
		c.logger.Errorf("Error processing %s (giving up): %v", newEvent.(Event).key, err)
		c.queue.Forget(newEvent)
		utilruntime.HandleError(err)
	}

	return true
}

/* TODOs
- Enhance event creation using client-side cacheing machanisms - pending
- Enhance the processItem to classify events - done
- Send alerts correspoding to events - done
*/

func (c *Controller) processItem(newEvent Event) error {
	obj, _, err := c.informer.GetIndexer().GetByKey(newEvent.key)
	if err != nil {
		return fmt.Errorf("Error fetching object with key %s from store: %v", newEvent.key, err)
	}
	// get object's metedata
	objectMeta := utils.GetObjectMetaData(obj)

	// namespace retrived from event key incase namespace value is empty
	if newEvent.namespace == "" {
		newEvent.namespace = strings.Split(newEvent.key, "/")[0]
	}

	// process events based on its type
	switch newEvent.eventType {
	case "create":
		// compare CreationTimestamp and serverStartTime and alert only on latest events
		// Could be Replaced by using Delta or DeltaFIFO
		if objectMeta.CreationTimestamp.Sub(serverStartTime).Seconds() > 0 {
			if _, ok := global[newEvent.resourceType]; ok {
				c.eventHandler.ObjectCreated(obj)
			} else if _, ok := create[newEvent.resourceType]; ok {
				c.eventHandler.ObjectCreated(obj)
			}
			return nil
		}
	case "update":
		/* TODOs
		- enahace update event processing in such a way that, it send alerts about what got changed.
		*/
		kbEvent := event.Event{
			Kind:      newEvent.resourceType,
			Name:      newEvent.key,
			Namespace: newEvent.namespace,
		}
		if _, ok := global[newEvent.resourceType]; ok {
			c.eventHandler.ObjectUpdated(obj, kbEvent)
		} else if _, ok := update[newEvent.resourceType]; ok {
			c.eventHandler.ObjectUpdated(obj, kbEvent)
		}
		return nil
	case "delete":
		kbEvent := event.Event{
			Kind:      newEvent.resourceType,
			Name:      newEvent.key,
			Namespace: newEvent.namespace,
		}
		if _, ok := global[newEvent.resourceType]; ok {
			c.eventHandler.ObjectDeleted(kbEvent)
		} else if _, ok := delete[newEvent.resourceType]; ok {
			c.eventHandler.ObjectDeleted(kbEvent)
		}
		return nil
	}
	return nil
}

// loadEventConfig loads event list from Event config for granular alerting
func loadEventConfig(c *config.Config) {

	// Load Global events
	if len(c.Event.Global) > 0 {
		global = make(map[string]uint8)
		for _, r := range c.Event.Global {
			global[r] = 0
		}
	}

	// Load Create events
	if len(c.Event.Create) > 0 {
		create = make(map[string]uint8)
		for _, r := range c.Event.Create {
			create[r] = 0
		}
	}

	// Load Update events
	if len(c.Event.Update) > 0 {
		update = make(map[string]uint8)
		for _, r := range c.Event.Update {
			update[r] = 0
		}
	}

	// Load Delete events
	if len(c.Event.Delete) > 0 {
		delete = make(map[string]uint8)
		for _, r := range c.Event.Delete {
			delete[r] = 0
		}
	}
}
