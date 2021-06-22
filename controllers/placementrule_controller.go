/*
Copyright 2021.

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

	appsv1alpha1 "github.com/DanielXLee/clusternet-placementrule-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PlacementRuleReconciler reconciles a PlacementRule object
type PlacementRuleReconciler struct {
	client.Client
	AuthClient kubernetes.Interface
	Scheme     *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.open-cluster-management.io,resources=placementrules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.open-cluster-management.io,resources=placementrules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.open-cluster-management.io,resources=placementrules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PlacementRule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *PlacementRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the PlacementRule instance
	instance := &appsv1alpha1.PlacementRule{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)

	klog.Info("Reconciling:", req.NamespacedName, " with Get err:", err)

	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	orgclmap := make(map[string]string)
	for _, cl := range instance.Status.Decisions {
		orgclmap[cl.ClusterName] = cl.ClusterNamespace
	}
	// do nothing if has finalizer
	if len(instance.GetObjectMeta().GetFinalizers()) != 0 {
		return reconcile.Result{}, nil
	}

	// do nothing if not using mcm as scheduler (user set it to something else)
	scname := instance.Spec.SchedulerName
	if scname != "" && scname != appsv1alpha1.SchedulerNameDefault && scname != appsv1alpha1.SchedulerNameMCM {
		return reconcile.Result{}, nil
	}

	err = r.hubReconcile(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	updated := false

	for _, cl := range instance.Status.Decisions {
		ns, ok := orgclmap[cl.ClusterName]
		if !ok || ns != cl.ClusterNamespace {
			updated = true
			break
		}

		delete(orgclmap, cl.ClusterName)
	}

	if !updated && len(orgclmap) > 0 {
		updated = true
	}

	// reconcile finished check if need to upadte the resource
	if updated {
		klog.Info("Update placementrule ", instance.Name, " with decisions: ", instance.Status.Decisions)
		err = r.Status().Update(context.TODO(), instance)
	}

	klog.V(1).Info("Reconciling - finished.", req.NamespacedName, " with Get err:", err)

	return reconcile.Result{}, nil
}

func (r *PlacementRuleReconciler) UpdateStatus(request reconcile.Request, instance *appsv1alpha1.PlacementRule) error {
	err := r.Status().Update(context.TODO(), instance)

	if err != nil {
		klog.Error("Error returned when updating placementrule decisions:", err, " ,instance:", instance)
	}

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlacementRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.PlacementRule{}).
		Complete(r)
}
