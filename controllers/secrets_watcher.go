package controllers

import (
	"fmt"
	"time"

	"secrets-sync.operators.infra/types"

	V1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type controller types.Controller

// NewController creates a new Controller.
func NewController(clientSet *kubernetes.Clientset, configSet types.ConfigSet) *controller {
	return &controller{
		ClientSet: clientSet,
		ConfigSet: configSet,
	}
}

func (c *controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.Queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two secrets with the same key are never processed in
	// parallel.
	defer c.Queue.Done(key)

	// Invoke the method containing the business logic
	err := c.secretsSync(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

// secretsSync is the business logic of the controller. This controller, selects the secrets from the config
// that controller must copy from src to dst namespaces.
// If an error occurs, it should simply return the error.
func (c *controller) secretsSync(key string) error {
	obj, exists, err := c.Indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		// Select secret by event: deleted
		if err := c.deleteDstSecrets(key); err != nil {
			return err
		}
	} else {
		// Note that you also have to check the uid if you have a local controlled resource, which
		// is dependent on the actual instance, to detect that a Secret was recreated with the same name
		if err := c.createUpdateDstSecrets(obj); err != nil {
			return err
		}
	}

	return nil
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.Queue.Forget(key)
		return
	}

	// This controller retries 10 times if something goes wrong. After that, it stops trying.
	if c.Queue.NumRequeues(key) < 10 {
		klog.Infof("Error syncing secret %v: %v\n", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.Queue.AddRateLimited(key)
		return
	}

	c.Queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	klog.Infof("Dropping secret %q out of the queue: %v\n", key, err)
}

// Run begins watching and syncing.
func (c *controller) Run(workers int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.Queue.ShutDown()
	klog.Info("Starting Secrets controller")

	go c.Informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.Informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync\n"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping Secrets controller")
}

func (c *controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *controller) WatchSecrets() *controller {
	secretsListWatcher := cache.NewListWatchFromClient(c.ClientSet.CoreV1().RESTClient(),
		V1.ResourceSecrets.String(), V1.NamespaceAll, fields.AndSelectors(
			fields.OneTermNotEqualSelector(types.OneTermNotEqualKey, V1.NamespaceDefault),
			fields.OneTermNotEqualSelector(types.OneTermNotEqualKey, types.NamespaceKubeSystem),
			fields.OneTermNotEqualSelector(types.OneTermNotEqualKey, V1.NamespaceNodeLease),
		))

	// create the workqueue
	c.Queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Bind the workqueue to a cache with the help of an informer. This way we make sure that
	// whenever the cache is updated, the secret key is added to the workqueue.
	// Note that when we finally process the item from the workqueue, we might see a newer version
	// of the Secret than the version which was responsible for triggering the update.
	c.Indexer, c.Informer = cache.NewIndexerInformer(
		secretsListWatcher,
		&V1.Secret{},
		c.Timeout,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					c.Queue.Add(key)
				}
			},
			UpdateFunc: func(old interface{}, new interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err == nil {
					c.Queue.Add(key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				// IndexerInformer uses a delta queue, therefore for deletes we have to use this
				// key function.
				key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
				if err == nil {
					c.Queue.Add(key)
				}
			},
		},
		cache.Indexers{},
	)

	return c
}
