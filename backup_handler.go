package main

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	cbv1alpha1 "github.com/ryo-watanabe/k8s-backup/pkg/apis/clusterbackup/v1alpha1"
)

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runBackupWorker() {
	for c.processNextBackupItem(false) {
	}
}

func (c *Controller) runBackupQueuer() {
	for c.processNextBackupItem(true) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextBackupItem(queueonly bool) bool {
	// Proccess backup queue
	obj, shutdown := c.backupQueue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer c.backupQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.backupQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := c.backupSyncHandler(key, queueonly); err != nil {
			c.backupQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.backupQueue.Forget(obj)
		klog.V(4).Infof("Successfully synced '%s'", key)
		return nil
	}(obj)
	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// backupSyncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Backup resource
// with the current status of the resource.
func (c *Controller) backupSyncHandler(key string, queueonly bool) error {

	//getOptions := metav1.GetOptions{IncludeUninitialized: false}

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Backup resource with this namespace/name.
	backup, err := c.backupLister.Backups(namespace).Get(name)

	// if deleted.
	if err != nil {
		if errors.IsNotFound(err) {
			// TODO delete backhp data.

			// In deleting, exit sync handler here.
			return nil
		} else {
			return err
		}
	}

	if !queueonly && backup.Status.Phase == "InQueue" {
		// do backup
		backup, err = c.updateBackupStatus(backup, "InProgress", "")
		if err != nil {
			return err
		}
		time.Sleep(120 * time.Second)
		backup, err = c.updateBackupStatus(backup, "Completed", "")
		if err != nil {
			return err
		}
	}

	if backup.Status.Phase == "" {
		backup, err = c.updateBackupStatus(backup, "InQueue", "")
		if err != nil {
			return err
		}
	}

	c.recorder.Event(backup, corev1.EventTypeNormal, "Synced", "Backup synced successfully")
	return nil
}

func (c *Controller) updateBackupStatus(backup *cbv1alpha1.Backup, phase, reason string) (*cbv1alpha1.Backup, error) {
	backupCopy := backup.DeepCopy()
	backupCopy.Status.Phase = phase
	backupCopy.Status.Reason = reason
	backup, err := c.cbclientset.CustomerclusterV1alpha1().Backups(backup.Namespace).Update(backupCopy)
	if err != nil {
		return backup, fmt.Errorf("Failed to update backup status for " + backup.ObjectMeta.Name + " : " + err.Error())
	}
	return backup, err
}

// enqueueBackup takes a Backup resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Backup.
func (c *Controller) enqueueBackup(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.backupQueue.AddRateLimited(key)
}
