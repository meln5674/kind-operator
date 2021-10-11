package model

import (
	kindmeln5674v0 "github.com/meln5674/kind-operator.git/api/v0"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeconfig "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
)

func ClusterSecretMeta(cluster *kindmeln5674v0.Cluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}
}

func ClusterSecret(cluster *kindmeln5674v0.Cluster, cfg *kubeconfig.Config, dockerCA, dockerClientCert, dockerClientKey []byte) (corev1.Secret, error) {
	// https://github.com/kubernetes-sigs/kind/blob/v0.11.1/pkg/cluster/internal/kubeconfig/internal/kubeconfig/helpers.go#L25
	clusterName, err := KindClusterName(cluster)
	if err != nil {
		return corev1.Secret{}, err
	}
	contextName := "kind-" + clusterName
	var cfgClusterName string
	for _, context := range cfg.Contexts {
		if context.Name == contextName {
			cfgClusterName = context.Context.Cluster
		}
	}
	for ix := range cfg.Clusters {
		if cfg.Clusters[ix].Name == cfgClusterName {
			cfg.Clusters[ix].Cluster.Server = ClusterK8sAPIURL(cluster)
			cfg.Clusters[ix].Cluster.TLSServerName = ClusterHostname(cluster)
		}
	}

	cfgBytes, err := yaml.Marshal(&cfg)

	if err != nil {
		return corev1.Secret{}, err
	}
	return corev1.Secret{
		ObjectMeta: ClusterSecretMeta(cluster),
		Data: map[string][]byte{
			"config":   cfgBytes,
			"ca.pem":   dockerCA,
			"cert.pem": dockerClientCert,
			"key.pem":  dockerClientKey,
		},
	}, nil
}
