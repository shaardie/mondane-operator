/*
Copyright 2021 Sven Haardiek <sven@haardiek.de>.

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
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"

	mondaneorgv1alpha1 "github.com/shaardie/mondane-operator/api/v1alpha1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mondane.org,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mondane.org,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mondane.org,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the User object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	user := &mondaneorgv1alpha1.User{}
	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, err
	}

	err = r.ReconsileServiceMonitor(ctx, user)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	err = r.ReconsilePrometheusRule(ctx, user)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	err = r.ReconsileAlertmanagerConfig(ctx, user)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (r *UserReconciler) ReconsileServiceMonitor(ctx context.Context, user *mondaneorgv1alpha1.User) error {
	oldServiceMonitor := &monitoringv1.ServiceMonitor{}
	newServiceMonitor, err := r.serviceMonitorFromUser(ctx, user)
	if err != nil {
		return err
	}
	err = r.Get(ctx, types.NamespacedName{Namespace: user.Namespace, Name: user.Name}, oldServiceMonitor)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		err = r.Create(ctx, newServiceMonitor)
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(oldServiceMonitor.Spec, newServiceMonitor.Spec) {
		oldServiceMonitor.Spec = newServiceMonitor.Spec
		err = r.Update(ctx, oldServiceMonitor)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *UserReconciler) ReconsilePrometheusRule(ctx context.Context, user *mondaneorgv1alpha1.User) error {
	oldPrometheusRule := &monitoringv1.PrometheusRule{}
	newPrometheusRule, err := r.prometheusruleFromUser(ctx, user)
	if err != nil {
		return err
	}
	err = r.Get(ctx, types.NamespacedName{Namespace: user.Namespace, Name: user.Name}, oldPrometheusRule)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		err = r.Create(ctx, newPrometheusRule)
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(oldPrometheusRule.Spec, newPrometheusRule.Spec) {
		oldPrometheusRule.Spec = newPrometheusRule.Spec
		err = r.Update(ctx, oldPrometheusRule)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *UserReconciler) ReconsileAlertmanagerConfig(ctx context.Context, user *mondaneorgv1alpha1.User) error {
	oldAlertmanagerConfig := &monitoringv1alpha1.AlertmanagerConfig{}
	newAlertmanagerConfig, err := r.alertmanagerConfigFromUser(ctx, user)
	if err != nil {
		return err
	}
	err = r.Get(ctx, types.NamespacedName{Namespace: user.Namespace, Name: user.Name}, oldAlertmanagerConfig)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		err = r.Create(ctx, newAlertmanagerConfig)
		if err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(oldAlertmanagerConfig.Spec, newAlertmanagerConfig.Spec) {
		oldAlertmanagerConfig.Spec = newAlertmanagerConfig.Spec
		err = r.Update(ctx, oldAlertmanagerConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *UserReconciler) serviceMonitorFromUser(ctx context.Context, user *mondaneorgv1alpha1.User) (*monitoringv1.ServiceMonitor, error) {
	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name,
			Namespace: user.Namespace,
			Labels: map[string]string{
				"mondane.org/watch": "true",
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			JobLabel: "blackbox-exporter",
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{"mondane-user"},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": "blackbox-exporter",
					"app.kubernetes.io/name":     "prometheus-blackbox-exporter",
				},
			},
		},
	}
	for _, url := range user.Spec.URLs {
		sm.Spec.Endpoints = append(sm.Spec.Endpoints, monitoringv1.Endpoint{
			Interval: "15s",
			MetricRelabelConfigs: []*monitoringv1.RelabelConfig{
				{Replacement: url, TargetLabel: "instance"},
				{Replacement: user.Name, TargetLabel: "user"},
			},
			Params: map[string][]string{
				"module": {"http_2xx"},
				"target": {url},
			},
			Path:          "/probe",
			Port:          "http",
			Scheme:        "http",
			ScrapeTimeout: "15s",
		})
	}
	return sm, controllerutil.SetControllerReference(user, sm, r.Scheme)
}

func (r *UserReconciler) prometheusruleFromUser(ctx context.Context, user *mondaneorgv1alpha1.User) (*monitoringv1.PrometheusRule, error) {
	pr := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name,
			Namespace: user.Namespace,
			Labels: map[string]string{
				"mondane.org/watch": "true",
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "Webside down",
							// Security Problem
							Expr: intstr.FromString(fmt.Sprintf("probe_success{user=\"%v\"} == 0", user.Name)),
							For:  "5m",
						},
					},
				},
			},
		},
	}
	return pr, controllerutil.SetControllerReference(user, pr, r.Scheme)
}

func (r *UserReconciler) alertmanagerConfigFromUser(ctx context.Context, user *mondaneorgv1alpha1.User) (*monitoringv1alpha1.AlertmanagerConfig, error) {
	ac := &monitoringv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name,
			Namespace: user.Namespace,
			Labels: map[string]string{
				"mondane.org/watch": "true",
			},
		},
		Spec: monitoringv1alpha1.AlertmanagerConfigSpec{
			Route: &monitoringv1alpha1.Route{
				GroupBy: []string{"instance"},
				Matchers: []monitoringv1alpha1.Matcher{
					{
						Name:  "user",
						Value: user.Name,
					},
				},
				Receiver: user.Name,
			},
			Receivers: []monitoringv1alpha1.Receiver{
				{
					Name: user.Name,
					EmailConfigs: []monitoringv1alpha1.EmailConfig{
						{
							To: user.Spec.Email,
						},
					},
				},
			},
		},
	}
	return ac, controllerutil.SetControllerReference(user, ac, r.Scheme)
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mondaneorgv1alpha1.User{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Owns(&monitoringv1alpha1.AlertmanagerConfig{}).
		Complete(r)
}
