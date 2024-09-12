package controllers

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	leaderworkerset "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

func (r *LeaderWorkerSetReconciler) reconcileHPA(ctx context.Context, lws *leaderworkerset.LeaderWorkerSet) error {

	log := ctrl.LoggerFrom(ctx)

	if lws.Spec.ScalePolicy == nil || lws.Spec.ScalePolicy.Metrics == nil {
		log.V(1).Info(
			"No ScalePolicy or Metric is specified, skipping HPA reconciling process")
		return nil
	}

	current := &autoscalingv2.HorizontalPodAutoscaler{}

	// Get the expected HPA.
	expected, err := generateHPA(lws, r.Scheme)
	if err != nil {
		return err
	}

	err = r.Get(ctx, client.ObjectKeyFromObject(expected), current)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create the new HPA.
			log.V(1).Info("Creating HPA", "namespace", expected.Namespace, "name", expected.Name)
			return r.Create(ctx, expected)
		}
		return err
	}

	if !equality.Semantic.DeepEqual(expected.Spec, current.Spec) {
		log.V(1).Info("Updating HPA", "namespace", current.Namespace, "name", current.Name)
		expected.ResourceVersion = current.ResourceVersion
		err = r.Update(ctx, expected)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateHPA(lws *leaderworkerset.LeaderWorkerSet, scheme *runtime.Scheme) (
	*autoscalingv2.HorizontalPodAutoscaler, error) {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lws.Name,
			Namespace: lws.Namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       lws.Kind,
				Name:       lws.Name,
				APIVersion: lws.APIVersion,
			},
			MinReplicas: lws.Spec.ScalePolicy.MinReplicas,
			MaxReplicas: *lws.Spec.ScalePolicy.MaxReplicas,
			Metrics:     lws.Spec.ScalePolicy.Metrics,
		},
	}
	if err := controllerruntime.SetControllerReference(lws, hpa, scheme); err != nil {
		return nil, err
	}
	return hpa, nil
}
