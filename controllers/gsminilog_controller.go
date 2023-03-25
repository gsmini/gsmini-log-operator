/*
Copyright 2023.

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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/gsmini/gsmini-log-operator/api/v1"
	gsminiv1 "github.com/gsmini/gsmini-log-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GsminiLogReconciler reconciles a GsminiLog object
type GsminiLogReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.gsmini.cn,resources=gsminilogs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.gsmini.cn,resources=gsminilogs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.gsmini.cn,resources=gsminilogs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GsminiLog object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *GsminiLogReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &gsminiv1.GsminiLog{}

	klog.Infof("[Reconcile call  start][ns:%v][GsminiLog:%v]", req.Namespace, req.Name)
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Errorf("[ Reconcile start missing be deleted][ns:%v][GsminiLog:%v]", req.Namespace, req.Name)
			// 如果错误是不存在，那么可能是到调谐这里 就被删了
			return reconcile.Result{}, nil
		}
		// 其它错误打印一下
		klog.Errorf("[ Reconcile start other error][err:%v][ns:%v][GsminiLog:%v]", err, req.Namespace, req.Name)
		return reconcile.Result{}, err
	}
	fmt.Println(instance.Spec)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GsminiLogReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.GsminiLog{}).
		Complete(r)
}
