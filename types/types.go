package types

import (
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	NamespaceKubeSystem       = "kube-system"
	SyncAtAnnotation          = "secrets-sync.operators.infra/sync-at"
	SyncFromVersionAnnotation = "secrets-sync.operators.infra/sync-from-version"
	SyncKeysAnnotation        = "secrets-sync.operators.infra/sync-keys"
	OneTermNotEqualKey        = "metadata.namespace"
)

type Controller struct {
	ClientSet *kubernetes.Clientset
	ConfigSet
	Indexer  cache.Indexer
	Informer cache.Controller
	Queue    workqueue.RateLimitingInterface
}

type ConfigSet struct {
	KubeConfig string
	ConfigPath string
	SecretList struct {
		Secrets []Secret `yaml:"secrets"`
	}
	Master   string
	Version  bool
	Timeout  time.Duration
	Operator OperatorInfo
}

type Secret struct {
	Name          string   `yaml:"name"`
	SrcNamespace  string   `yaml:"src-namespace"`
	DstNamespaces []string `yaml:"dst-namespaces"`
}

type OperatorInfo struct {
	BuiltBy   string
	Commit    string
	Date      string
	Name      string
	Target    string
	Timestamp string
	Version   string
}
