package config

import (
	"context"
	"fmt"
	"os"

	"github.com/smou/k8s-csi-s3/pkg/driver/version"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	defaultRegion = "us-east-1"

	var_endpoint       = "MINIO_ENDPOINT"
	var_region         = "MINIO_REGION"
	var_bucketprefix   = "MINIO_BUCKET_PREFIX"
	var_accessKey      = "MINIO_ACCESSKEY" // secret
	var_secretKey      = "MINIO_SECRETKEY" // secret
	var_namespace      = "NAMESPACE"       // env
	var_configmap_name = "CONFIGMAP_NAME"  // env
	var_secret_name    = "SECRET_NAME"     // env
)

type DriverConfig struct {
	Endpoint          string
	NodeID            string
	MountBinaryS3     string
	MountBinary       string
	KubernetesVersion string
	S3                S3Config
	S3Credentials     S3Credentials
	Meta              Meta
}

type S3Config struct {
	Endpoint     string
	UseTLS       bool
	Region       string
	BucketPrefix string
}

type S3Credentials struct {
	AccessKey string
	SecretKey string
}

type Meta struct {
	DriverName    string
	DriverVersion string
}

func InitConfig(ctx context.Context) (*DriverConfig, error) {
	klog.Infof("Initializing Config...")
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot create in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("cannot create kubernetes clientset: %w", err)
	}
	kubernetesVersion, err := kubernetesVersion(clientset)
	if err != nil {
		klog.Errorf("failed to get kubernetes version: %v", err)
	}
	namespace := os.Getenv(var_namespace)
	if namespace == "" {
		return nil, fmt.Errorf("%v not set", var_namespace)
	}
	configmap_name := os.Getenv(var_configmap_name)
	if configmap_name == "" {
		return nil, fmt.Errorf("%v not set", var_configmap_name)
	}
	secret_name := os.Getenv(var_secret_name)
	if secret_name == "" {
		return nil, fmt.Errorf("%v not set", var_secret_name)
	}
	s3Config, err := LoadControllerConfigMap(ctx, clientset, namespace, configmap_name)
	if err != nil {
		return nil, fmt.Errorf("cannot load configmap %v: %w", configmap_name, err)
	}
	s3Creds, err := LoadControllerCredentialsFromSecret(ctx, *clientset, namespace, secret_name)
	if err != nil {
		return nil, fmt.Errorf("cannot load secret %v: %w", secret_name, err)
	}
	v := version.GetVersion()
	klog.Infof("Config initialized.")
	klog.V(4).Infof("Namespace: %v", namespace)
	klog.V(4).Infof("S3: %v", s3Config)
	klog.V(4).Infof("S3 Cred: %v", s3Creds)
	return &DriverConfig{
		KubernetesVersion: kubernetesVersion,
		S3:                *s3Config,
		S3Credentials:     *s3Creds,
		Meta: Meta{
			DriverName:    v.DriverName,
			DriverVersion: v.DriverVersion,
		},
	}, nil
}

func (d *DriverConfig) LogVersionInfo() {
	version := version.GetVersion()
	klog.Infof("Driver version: %v, Git commit: %v, build date: %v, nodeID: %v, kubernetes version: %v",
		d.Meta.DriverVersion, version.GitCommit, version.BuildDate, d.NodeID, d.KubernetesVersion)
}

func kubernetesVersion(clientset *kubernetes.Clientset) (string, error) {
	version, err := clientset.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("cannot get kubernetes server version: %w", err)
	}

	return version.String(), nil
}

func LoadControllerConfigMap(ctx context.Context, client *kubernetes.Clientset, namespace, name string) (*S3Config, error) {
	klog.Infof("Initializing Configmap '%s' in namespace %s", name, namespace)
	cm, err := client.CoreV1().
		ConfigMaps(namespace).
		Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data := cm.Data
	cfg := &S3Config{
		Endpoint:     data[var_endpoint],
		UseTLS:       data[var_endpoint] == "true",
		Region:       data[var_region],
		BucketPrefix: data[var_bucketprefix],
	}

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("%v missing in ConfigMap", var_endpoint)
	}
	if cfg.Region == "" {
		klog.Infof("%v missing in ConfigMap. Use Default: %v", var_region, defaultRegion)
		cfg.Region = defaultRegion
	}
	return cfg, nil
}

func LoadControllerCredentialsFromSecret(ctx context.Context, client kubernetes.Clientset, namespace, name string) (*S3Credentials, error) {
	klog.Infof("Initializing Secret '%s' in namespace %s", name, namespace)
	sec, err := client.CoreV1().
		Secrets(namespace).
		Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data := sec.Data
	cfg := &S3Credentials{
		AccessKey: string(data[var_accessKey]),
		SecretKey: string(data[var_secretKey]),
	}

	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("invalid controller credentials secret")
	}

	return cfg, nil
}
