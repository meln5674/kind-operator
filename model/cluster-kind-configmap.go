package model

import (
	kindmeln5674v0 "github.com/meln5674/kind-operator.git/api/v0"
	dumbyaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

func ClusterKindConfigConfigMapMeta(cluster *kindmeln5674v0.Cluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}
}

func CompleteKindConfig(cluster *kindmeln5674v0.Cluster) ([]byte, error) {
	kindCluster := kindv1alpha4.Cluster{}
	if cluster.Spec.KindConfig != nil {
		if err := dumbyaml.Unmarshal([]byte(cluster.Spec.KindConfig), &kindCluster); err != nil {
			return nil, err
		}
	}

	if kindCluster.APIVersion == "" {
		kindCluster.APIVersion = "kind.x-k8s.io/v1alpha4"
	}
	if kindCluster.Kind == "" {
		kindCluster.Kind = "Cluster"
	}
	var err error
	if kindCluster.Name, err = KindClusterName(cluster); err != nil {
		return nil, err
	}
	if kindCluster.Networking.APIServerPort == 0 {
		kindCluster.Networking.APIServerPort = defaultK8sAPIPort
	}
	if kindCluster.Networking.APIServerAddress == "" {
		kindCluster.Networking.APIServerAddress = "0.0.0.0"
	}

	return dumbyaml.Marshal(&kindCluster)
}

func ClusterKindConfigConfigMap(cluster *kindmeln5674v0.Cluster) (corev1.ConfigMap, error) {
	cfgBytes, err := CompleteKindConfig(cluster)
	if err != nil {
		return corev1.ConfigMap{}, err
	}
	return corev1.ConfigMap{
		ObjectMeta: ClusterKindConfigConfigMapMeta(cluster),
		Data: map[string]string{
			"kind.yaml": string(cfgBytes),
		},
	}, nil
}
