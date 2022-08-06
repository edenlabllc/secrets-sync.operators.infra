package helpers

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"secrets-sync.operators.infra/types"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type configSet types.ConfigSet

func lookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return defaultVal
}

func lookupEnvOrDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		v, err := time.ParseDuration(val)
		if err != nil {
			klog.Fatalf("lookupEnvOrDuration[%s]: %v", key, err)
		}

		return v
	}

	return defaultVal
}

func lookupEnvOrInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.Atoi(val)
		if err != nil {
			klog.Fatalf("LookupEnvOrInt[%s]: %v", key, err)
		}

		return v
	}

	return defaultVal
}

func getConfig(fs *flag.FlagSet) []string {
	cfg := make([]string, 0, 10)
	fs.VisitAll(func(f *flag.Flag) {
		cfg = append(cfg, fmt.Sprintf("%s:%q", f.Name, f.Value.String()))
	})

	return cfg
}

func NewConfigSet(op *types.OperatorInfo) *configSet {
	return &configSet{
		Operator: *op,
	}
}

func (c *configSet) getKubeConfig() *rest.Config {
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&c.KubeConfig, "kubeconfig",
			lookupEnvOrString("KUBECONFIG", filepath.Join(home, ".kube", "config")),
			"(optional) absolute path to the kube config file [$KUBECONFIG]")
	} else {
		flag.StringVar(&c.KubeConfig, "kubeconfig", lookupEnvOrString("KUBECONFIG", ""),
			"absolute path to the kubeconfig file [$KUBECONFIG]")
	}

	flag.StringVar(&c.Master, "master", lookupEnvOrString("MASTER", ""),
		"master url [$MASTER]")
	flag.StringVar(&c.ConfigPath, "config-path", lookupEnvOrString("CONFIG_PATH", "secrets.yaml"),
		"secrets config path [$CONFIG_PATH]")
	c.Timeout = *flag.Duration("resync-period",
		lookupEnvOrDuration("RESYNC_PERIOD", time.Second*15),
		"(required) resync period k8s cache with client for update event [$RESYNC_PERIOD]")
	flag.BoolVar(&c.Version, "version", false,
		"(optional) operator version")

	flag.Parse()

	if len(c.ConfigPath) == 0 {
		klog.Fatalln(fmt.Errorf("Required flag -config-path is not set\n"))
	}

	data, err := ioutil.ReadFile(c.ConfigPath)
	if err != nil {
		klog.Fatalln(err)
	}

	if err := yaml.Unmarshal(data, &c.SecretList); err != nil {
		klog.Fatalln(err)
	}

	if c.Version {
		fmt.Printf("Name: %s\nVersion: %s\n", c.Operator.Name, c.Operator.Version)
		os.Exit(0)
	}

	config, err := clientcmd.BuildConfigFromFlags(c.Master, c.KubeConfig)
	if err != nil {
		klog.Fatalln(err)
	}

	return config
}

func (c *configSet) GetClientSet() (*kubernetes.Clientset, *configSet, error) {
	clientSet, err := kubernetes.NewForConfig(c.getKubeConfig())
	if err != nil {
		return nil, nil, err
	}

	return clientSet, c, nil
}
