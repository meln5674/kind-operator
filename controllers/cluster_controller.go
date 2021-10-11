/*
Copyright 2021 Andrew M Melnick.

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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	kindmeln5674v0 "github.com/meln5674/kind-operator.git/api/v0"
	"github.com/meln5674/kind-operator.git/model"
	"github.com/pkg/errors"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeconfig "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"os/exec"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
	"strings"
	"time"
)

const (
	clusterFinalizerName = "cluster.kind.meln5674.github.com/finalizer"
	kubeconfigExportPath = "/tmp/kube.config"
	dockerCAPath         = "/certs/client/ca.pem"
	dockerClientCertPath = "/certs/client/cert.pem"
	dockerClientKeyPath  = "/certs/client/key.pem"
	dockerCertPathEnv    = "DOCKER_CERT_PATH"
	dockerHostEnv        = "DOCKER_HOST"
)

type PodExecutor interface {
	Exec(context.Context, client.ObjectKey, corev1.PodExecOptions) (remotecommand.Executor, error)
}

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	PodExecutor
	Scheme       *runtime.Scheme
	RequeueDelay time.Duration
	KindPath     string
}

type ClusterReconcilerRun struct {
	*ClusterReconciler
	ctx                 context.Context
	log                 logr.Logger
	cluster             kindmeln5674v0.Cluster
	kubeconfig          *kubeconfig.Config
	dockerCA            []byte
	dockerClientCert    []byte
	dockerClientKey     []byte
	service             *corev1.Service
	statefulSet         *appsv1.StatefulSet
	pod                 *corev1.Pod
	kubeconfigSecret    *corev1.Secret
	kindConfigConfigMap *corev1.ConfigMap
}

func (r *ClusterReconcilerRun) kubeconfigSecretKey() client.ObjectKey {
	obj := model.ClusterSecretMeta(&r.cluster)
	return client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func (r *ClusterReconcilerRun) serviceKey() client.ObjectKey {
	obj := model.ClusterServiceMeta(&r.cluster)
	return client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func (r *ClusterReconcilerRun) statefulSetKey() client.ObjectKey {
	obj := model.ClusterStatefulSetMeta(&r.cluster)
	return client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func (r *ClusterReconcilerRun) kindConfigConfigMapKey() client.ObjectKey {
	obj := model.ClusterKindConfigConfigMapMeta(&r.cluster)
	return client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}

func (r *ClusterReconcilerRun) fetchResources(req ctrl.Request) error {
	if err := r.Get(r.ctx, req.NamespacedName, &r.cluster); err != nil {
		return err
	}
	r.log.Info("Fetched cluster")

	kubeconfigSecret := new(corev1.Secret)
	kubeconfigSecretKey := r.kubeconfigSecretKey()
	if err := r.Get(r.ctx, kubeconfigSecretKey, kubeconfigSecret); client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "Failed to fetch secret")
	} else if err == nil {
		r.log.Info("Fetched secret", "secret", kubeconfigSecretKey)
		r.kubeconfigSecret = kubeconfigSecret
	} else {
		r.log.Info("Secret does not exist yet", "secret", kubeconfigSecretKey)
	}

	service := new(corev1.Service)
	serviceKey := r.serviceKey()
	if err := r.Get(r.ctx, serviceKey, service); client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "Failed to fetch service")
	} else if err == nil {
		r.log.Info("Fetched service", "service", serviceKey)
		r.service = service
	} else {
		r.log.Info("Service does not exist yet", "service", serviceKey)
	}

	statefulSet := new(appsv1.StatefulSet)
	statefulSetKey := r.statefulSetKey()
	if err := r.Get(r.ctx, statefulSetKey, statefulSet); client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "Failed to fetch stateful set")
	} else if err == nil {
		r.log.Info("Fetched stateful set", "statefulset", statefulSetKey)
		r.statefulSet = statefulSet
	} else {
		r.log.Info("Stateful set does not exist yet", "statefulset", statefulSetKey)
	}

	if r.statefulSet != nil {
		pods := new(corev1.PodList)
		if err := r.List(r.ctx, pods, client.MatchingLabels(r.statefulSet.Spec.Selector.MatchLabels)); client.IgnoreNotFound(err) != nil {
			return errors.Wrap(err, "Failed to fetch stateful set pod")
		}
		if len(pods.Items) != 0 {
			r.log.Info("Found stateful set pod", "pod", client.ObjectKeyFromObject(&pods.Items[0]), "podCount", len(pods.Items))
			r.pod = new(corev1.Pod)
			*r.pod = pods.Items[0]
		} else {
			r.log.Info("No pods found for stateful set")
		}
	}

	kindConfigConfigMap := new(corev1.ConfigMap)
	kindConfigConfigMapKey := r.kindConfigConfigMapKey()
	if err := r.Get(r.ctx, kindConfigConfigMapKey, kindConfigConfigMap); client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "Failed to fetch config map")
	} else if err == nil {
		r.log.Info("Fetched config map", "configmap", kindConfigConfigMapKey)
		r.kindConfigConfigMap = kindConfigConfigMap
	} else {
		r.log.Info("Config map does not exist yet", "configmap", kindConfigConfigMapKey)
	}

	r.log.Info("Fetched all resources")

	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func (r *ClusterReconcilerRun) ensureResourcesDeleted() error {
	if r.kubeconfigSecret != nil {
		if err := r.Delete(r.ctx, r.kubeconfigSecret); err != nil {
			return errors.Wrap(err, "Failed to delete secret")
		}
		r.log.Info("Secret deleted")
	}

	if r.service != nil {
		if err := r.Delete(r.ctx, r.service); err != nil {
			return errors.Wrap(err, "Failed to delete service")
		}
		r.log.Info("Service deleted")
	}

	if r.statefulSet != nil {
		if err := r.Delete(r.ctx, r.statefulSet); err != nil {
			return errors.Wrap(err, "Failed to delete stateful set")
		}
		r.log.Info("Stateful set deleted")
	}

	if r.kindConfigConfigMap != nil {
		if err := r.Delete(r.ctx, r.kindConfigConfigMap); err != nil {
			return errors.Wrap(err, "Failed to delete config map")
		}
		r.log.Info("Failed to delete config map")
	}

	r.log.Info("All resources deleted")

	return nil
}

func (r *ClusterReconcilerRun) handleFinalizer() (deleted bool, err error) {
	if r.cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(r.cluster.GetFinalizers(), clusterFinalizerName) {
			controllerutil.AddFinalizer(&r.cluster, clusterFinalizerName)
			if err := r.Update(r.ctx, &r.cluster); err != nil {
				return false, errors.Wrap(err, "Failed to add finalizer")
			}
		}
		r.log.Info("Finalizer added")
	} else {
		r.log.Info("Cluster marked for deletion")
		if err := r.fetchDockerFiles(); err != nil {
			return false, errors.Wrap(err, "Failed to fetch docker files for deletion. You may need to manually delete this cluster's statefulset. Note that containers running in docker-in-docker may continue to run if this happens")
		}
		if containsString(r.cluster.GetFinalizers(), clusterFinalizerName) {
			if r.statefulSet != nil {
				r.log.Info("Stateful set stil exists, ensuring kind cluster doesn't exist")
				if err := r.ensureKindClusterDeleted(); err != nil {
					return false, errors.Wrap(err, "Failed to delete kind cluster")
				}
				r.log.Info("Kind cluster deleted")
			}

			if err := r.ensureResourcesDeleted(); err != nil {
				return false, errors.Wrap(err, "Failed to delete child resources")
			}

			controllerutil.RemoveFinalizer(&r.cluster, clusterFinalizerName)
			if err := r.Update(r.ctx, &r.cluster); err != nil {
				return false, errors.Wrap(err, "Failed to remove finalizer")
			}
		}
		r.log.Info("Cluster resource ready for deletion")

		return true, nil
	}

	return false, nil
}

func (r *ClusterReconcilerRun) ensureClusterKindConfigConfigMapExists() error {
	expected, err := model.ClusterKindConfigConfigMap(&r.cluster)
	if err != nil {
		return err
	}

	if r.kindConfigConfigMap == nil {
		r.log.Info("Creating config map")
		r.kindConfigConfigMap = &expected
		return r.Create(r.ctx, r.kindConfigConfigMap)
	}

	for key := range expected.Data {
		if r.kindConfigConfigMap.Data[key] != expected.Data[key] {
			r.kindConfigConfigMap.Data = expected.Data
			r.log.Info("Config map missing field, updating", "field", key)
			return r.Update(r.ctx, r.kindConfigConfigMap)
		}
	}
	r.log.Info("Config map up to date")

	return nil
}

func (r *ClusterReconcilerRun) ensureClusterServiceExists() error {
	expected := model.ClusterService(&r.cluster)

	if r.service == nil {
		r.log.Info("Creating service")
		r.service = &expected
		return r.Create(r.ctx, r.service)
	}

	outOfSync := false
	for key := range expected.Spec.Selector {
		if r.service.Spec.Selector[key] != expected.Spec.Selector[key] {
			r.service.Spec.Selector = expected.Spec.Selector
			outOfSync = true
			break
		}
	}
	if len(r.service.Spec.Ports) != len(r.service.Spec.Ports) {
		outOfSync = true
	} else {
		for ix := range r.service.Spec.Ports {
			if r.service.Spec.Ports[ix] != expected.Spec.Ports[ix] {
				outOfSync = true
				break
			}
		}
	}
	if outOfSync {
		r.log.Info("Service out of sync, updating")
		return r.Update(r.ctx, r.service)
	}
	r.log.Info("Service up to date")

	return nil
}

func (r *ClusterReconcilerRun) ensureClusterStatefulSetExists() error {
	expected := model.ClusterStatefulSet(&r.cluster)
	if r.statefulSet == nil {
		r.log.Info("Stateful set doesn't exist, creating it")
		r.statefulSet = &expected
		return r.Create(r.ctx, r.statefulSet)
	}
	if true { //r.statefulSet.Spec.Template.Spec != expected.Spec.Template.Spec || r.statefulSet.Spec.Replicas != expected.Spec.Replicas {
		r.statefulSet.Spec.Template.Spec = expected.Spec.Template.Spec
		r.statefulSet.Spec.Replicas = expected.Spec.Replicas
		r.log.Info("Stateful set out of sync, updating")
		return r.Update(r.ctx, r.statefulSet)
	}
	r.log.Info("Stateful set up to date")
	return nil
}

func (r *ClusterReconcilerRun) ensureClusterSecretExists() error {
	expected, err := model.ClusterSecret(&r.cluster, r.kubeconfig, r.dockerCA, r.dockerClientCert, r.dockerClientKey)
	if err != nil {
		return err
	}
	if r.kubeconfigSecret == nil {
		r.log.Info("Secret doesn't exist, creating")
		r.kubeconfigSecret = &expected
		return r.Create(r.ctx, r.kubeconfigSecret)
	}
	for key, _ := range expected.Data {
		if string(r.kubeconfigSecret.Data[key]) != string(expected.Data[key]) {
			r.log.Info("Secret missing field, updating", "field", key)
			r.kubeconfigSecret.Data = expected.Data
			return r.Update(r.ctx, r.kubeconfigSecret)
		}
	}
	r.log.Info("Secret up to date")
	return nil
}

func (r *ClusterReconcilerRun) ensureClusterStatefulSetLive() (live bool, err error) {
	if r.statefulSet.Status.Replicas == 0 || r.pod == nil {
		r.log.Info("Stateful set has no pods yet", "replicas", r.statefulSet.Status.Replicas)
		return false, nil
	}

	for _, cond := range r.pod.Status.Conditions {
		if cond.Type == corev1.PodInitialized && cond.Status == corev1.ConditionTrue {
			r.log.Info("Stateful set pod is Initialized")
			return true, nil
		}
	}
	r.log.Info("Stateful set pod is not yet initialized")

	return false, nil
}

func (r *ClusterReconcilerRun) execPosixScript(script, stdin string) (stdout, stderr string, err error) {
	r.log.Info("Executing script on cluster pod", "pod", r.pod, "script", script, "stdin", stdin)
	stdinWrapper := strings.NewReader(stdin)
	stdoutWrapper := strings.Builder{}
	stderrWrapper := strings.Builder{}
	exec, err := r.Exec(r.ctx, client.ObjectKeyFromObject(r.pod), corev1.PodExecOptions{
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
		Container: model.KindContainerName(&r.cluster),
		Command:   []string{"/bin/sh", "-c", script},
	})
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to execute posix shell script")
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdinWrapper,
		Stdout: &stdoutWrapper,
		Stderr: &stderrWrapper,
	})
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to stream from posix shell script")
	}
	stdout = stdoutWrapper.String()
	stderr = stderrWrapper.String()
	r.log.Info("Executed script on cluster pod", "pod", r.pod, "script", script, "stdin", stdin, "stdout", stdout, "stderr", stderr)
	return stdout, stderr, nil
}

func (r *ClusterReconcilerRun) kind(configSubPath string, args ...string) (stdout, stderr string, err error) {
	dir, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(dir)

	if configSubPath != "" {
		cfgBytes, err := model.CompleteKindConfig(&r.cluster)
		if err != nil {
			return "", "", err
		}
		if err := ioutil.WriteFile(filepath.Join(dir, configSubPath), cfgBytes, 0700); err != nil {
			return "", "", err
		}
	}
	if r.dockerCA != nil {
		if err := ioutil.WriteFile(filepath.Join(dir, "ca.pem"), r.dockerCA, 0700); err != nil {
			return "", "", err
		}
	}
	if r.dockerClientCert != nil {
		if err := ioutil.WriteFile(filepath.Join(dir, "cert.pem"), r.dockerClientCert, 0700); err != nil {
			return "", "", err
		}
	}
	if r.dockerClientKey != nil {
		if err := ioutil.WriteFile(filepath.Join(dir, "key.pem"), r.dockerClientKey, 0700); err != nil {
			return "", "", err
		}
	}
	r.log.Info("Executing kind command", "kind", r.KindPath, "args", args)
	cmd := exec.Command(r.KindPath, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", dockerCertPathEnv, dir))
	// We use the raw (pod) URL because the pod will not be marked ready until the k8s api is ready
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", dockerHostEnv, model.ClusterDockerRawURL(&r.cluster, r.pod)))
	cmd.Env = append(cmd.Env, "DOCKER_TLS_VERIFY=yes")
	stdoutWrapper := strings.Builder{}
	stderrWrapper := strings.Builder{}
	cmd.Stdout = &stdoutWrapper
	cmd.Stderr = &stderrWrapper
	cmd.Dir = dir
	err = cmd.Run()
	stdout = stdoutWrapper.String()
	stderr = stderrWrapper.String()
	r.log.Info("Kind command finished", "stdout", stdout, "stderr", stderr, "err", err)
	return
}

func (r *ClusterReconcilerRun) fetchFileFromCluster(path string) (contents string, err error) {
	stdout, stderr, err := r.execPosixScript(
		fmt.Sprintf(
			`
			set -e
			cat '%s'
			echo yes 1>&2
			`,
			path,
		),
		"",
	)
	if err != nil {
		return "", err
	}

	if stderr != "yes\n" {
		err = fmt.Errorf("Did not get expected result from fetching file.\nBEGIN STDOUT\n%s\nEND STDOUT\nBEGIN STDERR\n%s\nEND STDERR", stdout, stderr)
		return
	}
	contents = stdout
	return
}

func (r *ClusterReconcilerRun) fetchDockerFiles() error {
	dockerCA, err := r.fetchFileFromCluster(dockerCAPath)
	if err != nil {
		return err
	}

	dockerClientCert, err := r.fetchFileFromCluster(dockerClientCertPath)
	if err != nil {
		return err
	}

	dockerClientKey, err := r.fetchFileFromCluster(dockerClientKeyPath)
	if err != nil {
		return err
	}

	r.dockerCA = []byte(dockerCA)
	r.dockerClientCert = []byte(dockerClientCert)
	r.dockerClientKey = []byte(dockerClientKey)

	return nil
}

func (r *ClusterReconcilerRun) checkClusterExists(clusterName string) (bool, error) {
	clusterList, stderr, err := r.kind("", "get", "clusters")
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("Failed to list kind clusters: %s", stderr))
	}
	clusters := strings.Split(clusterList, "\n")
	r.log.Info("Found clusters", "clusters", clusters)
	for _, name := range clusters {
		if name == clusterName {
			r.log.Info("Kind cluster exists")
			return true, nil
		}
	}
	r.log.Info("Kind cluster doesn't exist")
	return false, nil
}

func (r *ClusterReconcilerRun) ensureKindClusterCreated() error {
	clusterName, err := model.KindClusterName(&r.cluster)
	if err != nil {
		return err
	}

	clusterExists, err := r.checkClusterExists(clusterName)
	if err != nil {
		return errors.Wrap(err, "Failed to check if kind cluser exists")
	}
	f, err := os.CreateTemp(os.TempDir(), "kubeconfig-*.yaml")
	if err != nil {
		return err
	}
	f.Close()
	defer os.Remove(f.Name())

	if !clusterExists {
		r.log.Info("Kind cluser doesn't exist, creating")
		configPath := "kind.yaml"
		args := []string{"create", "cluster", "--kubeconfig", f.Name(), "--config", configPath}
		if stdout, stderr, err := r.kind(configPath, args...); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to create cluster. Stdout: %s, Stderr: %s", stdout, stderr))
		}
	} else {
		r.log.Info("Kind cluser exists, exporting kubeconfig for sync")
		if stdout, stderr, err := r.kind("", "export", "kubeconfig", "--name", clusterName, "--kubeconfig", f.Name()); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to export kubeconfig. Stdout: %s, Stderr: %s", stdout, stderr))
		}
	}
	kubeconfigBytes, err := ioutil.ReadFile(f.Name())
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(kubeconfigBytes, &r.kubeconfig); err != nil {
		return err
	}
	return nil
}

func (r *ClusterReconcilerRun) ensureKindClusterDeleted() error {
	clusterName, err := model.KindClusterName(&r.cluster)
	if err != nil {
		return err
	}
	clusterExists, err := r.checkClusterExists(clusterName)
	if err != nil {
		return errors.Wrap(err, "Failed to check if kind cluster exists")
	}
	if clusterExists {
		r.log.Info("Kind cluster exists, deleting")
		if stdout, stderr, err := r.kind("", "delete", "cluster", "--name", clusterName); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to delete cluster. Stdout: %s, Stderr: %s", stdout, stderr))
		}
	}
	r.log.Info("Kind cluster deleted")
	return nil
}

//+kubebuilder:rbac:groups=kind.meln5674,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kind.meln5674,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kind.meln5674,resources=clusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=,resources=secrets;configmaps;services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=,resources=pods/exec,verbs=create,get
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	run := ClusterReconcilerRun{
		ClusterReconciler: r,
		log:               log.FromContext(ctx).WithValues("cluster", req),
		ctx:               ctx,
		cluster:           kindmeln5674v0.Cluster{},
	}

	run.log.Info("Reconciling")

	if err := run.fetchResources(req); err != nil {
		if client.IgnoreNotFound(err) == nil {
			run.log.Info("Got a request for a cluster that doesn't exist, assuming deleted")
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	if deleted, err := run.handleFinalizer(); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	} else if deleted {
		return ctrl.Result{Requeue: false}, nil
	}

	if err := run.ensureClusterKindConfigConfigMapExists(); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	if err := run.ensureClusterServiceExists(); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	if err := run.ensureClusterStatefulSetExists(); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	if live, err := run.ensureClusterStatefulSetLive(); err != nil || !live {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	if err := run.fetchDockerFiles(); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	if err := run.ensureKindClusterCreated(); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	if err := run.ensureClusterSecretExists(); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueDelay}, err
	}

	run.log.Info("Cluster ready")
	return ctrl.Result{Requeue: false}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kindmeln5674v0.Cluster{}).
		Complete(r)
}
