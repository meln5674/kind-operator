package model

import (
	kindv0 "github.com/meln5674/kind-operator.git/api/v0"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"path/filepath"
)

const (
	defaultKindContainerName = "kind"
	defaultKindImage         = "docker:dind"
	dockerVolumeName         = "docker"
	//dockerVolumeClaimNameSuffix      = "-docker"
	defaultDockerVolumeMount = "/var/lib/docker"
	provisionerVolumeName    = "pvcs"
	//provisionerVolumeClaimNameSuffix = "-pvcs"
	defaultProvisionerVolumeMount = "/var/local-path-provisioner"
	k8sAPIPortName                = "api-server"
	dockerPortName                = "docker"
	kindConfigVolumeName          = "kind-config"
	KindConfigMountPath           = "/etc/kind/config.yaml"
)

var (
	defaultDockerVolumeStorage      = resource.MustParse("8Gi")
	defaultProvisionerVolumeStorage = resource.MustParse("8Gi")
)

func ClusterStatefulSetMeta(cluster *kindv0.Cluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
		Labels:    cluster.ObjectMeta.Labels,
	}
}

func ClusterStatefulSet(cluster *kindv0.Cluster) appsv1.StatefulSet {

	podSpec := corev1.PodSpec{}
	if cluster.Spec.PodSpec != nil {
		podSpec = *cluster.Spec.PodSpec
	}

	kindContainerName := KindContainerName(cluster)

	var kindContainer *corev1.Container
	for ix := range podSpec.Containers {
		if podSpec.Containers[ix].Name == kindContainerName {
			kindContainer = &podSpec.Containers[ix]
		}
	}
	if kindContainer == nil {
		podSpec.Containers = append(podSpec.Containers, corev1.Container{
			Name: kindContainerName,
		})
		kindContainer = &podSpec.Containers[len(podSpec.Containers)-1]
	}
	if kindContainer.Image == "" {
		kindContainer.Image = defaultKindImage
	}

	// TODO: Find a way around this

	if kindContainer.SecurityContext == nil {
		kindContainer.SecurityContext = &corev1.SecurityContext{}
	}
	kindContainer.SecurityContext.Privileged = new(bool)
	*kindContainer.SecurityContext.Privileged = true

	k8sAPIPort := ClusterK8sAPIPort(cluster)
	dockerPort := ClusterDockerPort(cluster)

	hasDockerPort := false
	hasAPIServerPort := false
	for ix := range kindContainer.Ports {
		if kindContainer.Ports[ix].Name == k8sAPIPortName {
			hasAPIServerPort = true
			if hasDockerPort {
				break
			}
		}
		if kindContainer.Ports[ix].ContainerPort == dockerPort {
			hasDockerPort = true
			if hasAPIServerPort {
				break
			}
		}
	}
	if !hasAPIServerPort {
		kindContainer.Ports = append(kindContainer.Ports, corev1.ContainerPort{
			Name:          k8sAPIPortName,
			ContainerPort: k8sAPIPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}
	if !hasDockerPort {
		kindContainer.Ports = append(kindContainer.Ports, corev1.ContainerPort{
			Name:          dockerPortName,
			ContainerPort: dockerPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}
	hasDockerVolumeMount := false
	hasProvisionerVolumeMount := false
	for ix := range kindContainer.VolumeMounts {
		if kindContainer.VolumeMounts[ix].Name == dockerVolumeName {
			hasDockerVolumeMount = true
			if hasProvisionerVolumeMount {
				break
			}
		}
		if kindContainer.VolumeMounts[ix].Name == provisionerVolumeName {
			hasProvisionerVolumeMount = true
			if hasDockerVolumeMount {
				break
			}
		}
	}
	if !hasDockerVolumeMount {
		kindContainer.VolumeMounts = append(kindContainer.VolumeMounts, corev1.VolumeMount{
			Name:      dockerVolumeName,
			MountPath: defaultDockerVolumeMount,
		})
	}
	if !hasProvisionerVolumeMount {
		kindContainer.VolumeMounts = append(kindContainer.VolumeMounts, corev1.VolumeMount{
			Name:      provisionerVolumeName,
			MountPath: defaultProvisionerVolumeMount,
		})

	}
	kindContainer.VolumeMounts = append(kindContainer.VolumeMounts, corev1.VolumeMount{
		Name:      kindConfigVolumeName,
		MountPath: KindConfigMountPath,
		SubPath:   filepath.Base(KindConfigMountPath),
	})

	blank := corev1.Handler{}

	if kindContainer.LivenessProbe == nil {
		kindContainer.LivenessProbe = &corev1.Probe{}
	}
	if kindContainer.LivenessProbe.Handler == blank {
		kindContainer.LivenessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"docker", "ps"},
				},
			},
		}
	}

	if kindContainer.ReadinessProbe == nil {
		kindContainer.ReadinessProbe = &corev1.Probe{}
	}
	if kindContainer.ReadinessProbe.Handler == blank {
		kindContainer.ReadinessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Port:   intstr.FromString(k8sAPIPortName),
					Path:   "/readyz",
					Scheme: corev1.URISchemeHTTPS,
				},
			},
		}
	}
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: kindConfigVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: ClusterKindConfigConfigMapMeta(cluster).Name,
				},
			},
		},
	})

	volumeClaimTemplates := cluster.Spec.VolumeClaimTemplates

	hasDockerVolumeClaim := false
	hasProvisionerVolumeClaim := false
	for ix := range volumeClaimTemplates {
		if volumeClaimTemplates[ix].Name == dockerVolumeName {
			hasDockerVolumeClaim = true
			if hasProvisionerVolumeClaim {
				break
			}
		}
		if volumeClaimTemplates[ix].Name == provisionerVolumeName {
			hasProvisionerVolumeClaim = true
			if hasDockerVolumeClaim {
				break
			}
		}
	}
	if !hasDockerVolumeClaim {
		volumeClaimTemplates = append(volumeClaimTemplates, corev1.PersistentVolumeClaim{

			ObjectMeta: metav1.ObjectMeta{
				Name: dockerVolumeName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: defaultDockerVolumeStorage,
					},
				},
			},
		})

	}

	if !hasProvisionerVolumeClaim {
		volumeClaimTemplates = append(volumeClaimTemplates, corev1.PersistentVolumeClaim{

			ObjectMeta: metav1.ObjectMeta{
				Name: provisionerVolumeName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: defaultProvisionerVolumeStorage,
					},
				},
			},
		})

	}

	selector := ClusterPodLabelSelector(cluster)

	one := int32(1)
	return appsv1.StatefulSet{
		ObjectMeta: ClusterStatefulSetMeta(cluster),
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &one,
			Selector:    &metav1.LabelSelector{MatchLabels: selector},
			ServiceName: ClusterServiceMeta(cluster).Name,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selector,
				},
				Spec: podSpec,
			},
			VolumeClaimTemplates: volumeClaimTemplates,
		},
	}
}
