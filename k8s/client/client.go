package client

import (
	"flag"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
	"sync"
)

func loadFromKubeConfig() (*rest.Config, error) {
	log.Println("Attempt to load from config")
	var kubeConfigPath *string
	kubeConfigPath = flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), "wkube", "config"), "kube config file")
	flag.Parse()
	restCfg, restErr := clientcmd.BuildConfigFromFlags("", *kubeConfigPath)
	return restCfg, restErr
}

func assumeServiceAccountAccess() (*rest.Config, error) {
	log.Println("attempt to load from serviceaccount")
	return rest.InClusterConfig()
}

var clientSet *kubernetes.Clientset
var mut = sync.Mutex{}

func GetClient() *kubernetes.Clientset {
	if clientSet != nil {
		return clientSet
	}
	initialize()
	return clientSet
}

func initialize() {
	mut.Lock()
	defer mut.Unlock()

	var cfg *rest.Config
	var err error
	cfg, err = assumeServiceAccountAccess()
	if err != nil {
		log.Println("kubernetes service account error", err)
		cfg, err = loadFromKubeConfig()
	}
	if err != nil {
		panic(err.Error())
	}
	cset, cerr := kubernetes.NewForConfig(cfg)
	if cerr != nil {
		log.Println("error initialising kubernetes config", cerr)
		panic(cerr.Error())
	}
	if clientSet == nil {
		clientSet = cset
	}
}
