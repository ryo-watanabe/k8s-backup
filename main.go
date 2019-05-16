/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"flag"
	"time"
	//"os"
	//"io/ioutil"

	//kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/ryo-watanabe/k8s-backup/pkg/client/clientset/versioned"
	informers "github.com/ryo-watanabe/k8s-backup/pkg/client/informers/externalversions"
	"github.com/ryo-watanabe/k8s-backup/pkg/signals"
	//"github.com/ryo-watanabe/k8s-backup/pkg/objectstore"
)

var (
	masterURL  string
	kubeconfig string
	namespace string
	backupthreads int
	restorethreads int
	//awsregion string
	//awsendpoint string
	//awsaccesskey string
	//awssecretkey string
	//bucketname string
)

func main() {

	////// For client-go <= 8.0.0 must sync flags glog and klog.
	//flag.Set("logtostderr", "true")
	//klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	//klog.InitFlags(klogFlags)
	// Sync the glog and klog flags.
	//flag.CommandLine.VisitAll(func(f1 *flag.Flag) {
	//	f2 := klogFlags.Lookup(f1.Name)
	//	if f2 != nil {
	//		value := f1.Value.String()
	//		f2.Value.Set(value)
	//	}
	//})
	//flag.Parse()
	//klog.Info("Set logs output to stderr.")
	//klog.Flush()

	////// For client-go > 8.0 ??
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()
	klog.Info("Set logs output to stderr.")
	klog.Flush()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	cbClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building customercluster clientset: %s", err.Error())
	}

	// Objectstore config
	//if awsaccesskey == "" {
	//	awsaccesskey = os.Getenv("AWS_ACCESS_KEY")
	//}
	//if awssecretkey == "" {
	//	awssecretkey = os.Getenv("AWS_SECRET_KEY")
	//}
	//klog.Infof("access key:%s", awsaccesskey)
	//klog.Infof("secret key:%s", awssecretkey)
	//bucket := objectstore.NewBucket(awsaccesskey, awssecretkey, awsendpoint, awsregion, bucketname)

	//token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	//if err != nil {
	//	klog.Fatalf("Error reading token : %s", err.Error())
	//}

	cbInformerFactory := informers.NewSharedInformerFactory(cbClient, time.Second*30)

	controller := NewController(kubeClient, cbClient,
		cbInformerFactory.Clusterbackup().V1alpha1().Backups(),
		cbInformerFactory.Clusterbackup().V1alpha1().Restores(),
		namespace,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	cbInformerFactory.Start(stopCh)

	if err = controller.Run(backupthreads, restorethreads, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", "k8s-backup", "Namespace for k8s-backup")
	flag.IntVar(&backupthreads, "backupthreads", 5, "Number of backup threads")
	flag.IntVar(&restorethreads, "restorethreads", 2, "Number of restore threads")
	//flag.StringVar(&awsregion, "awsregion", "", "Set S3 Region")
	//flag.StringVar(&awsendpoint, "awsendpoint", "", "Set S3 Endpoint for S3-compatible object stores")
	//flag.StringVar(&awsaccesskey, "awsaccesskey", "", "Set S3 access key here or set environment variable AWS_ACCESS_KEY")
	//flag.StringVar(&awssecretkey, "awssecretkey", "", "Set S3 secret key here or set environment variable AWS_SECRET_KEY")
	//flag.StringVar(&bucketname, "bucketname", "", "S3 bucket name")
}
